package maintenance

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ProductionLogDirectory = "/var/log/iam"

// LogDisposalSummary contains metadata only; it never includes file names or
// file contents.
type LogDisposalSummary struct {
	FileCount              int        `json:"file_count"`
	TotalBytes             int64      `json:"total_bytes"`
	EarliestModifiedAt     *time.Time `json:"earliest_modified_at,omitempty"`
	LatestModifiedAt       *time.Time `json:"latest_modified_at,omitempty"`
	RefreshTokenLogMatches int64      `json:"refresh_token_log_matches"`
	GORMSQLLogMatches      int64      `json:"gorm_sql_log_matches"`
	DeletedFiles           int        `json:"deleted_files"`
}

// LogDisposalPlan keeps the selected paths private so callers cannot
// accidentally print them as part of the JSON summary.
type LogDisposalPlan struct {
	summary    LogDisposalSummary
	candidates []logCandidate
}

type logCandidate struct {
	path string
	info os.FileInfo
}

func (p *LogDisposalPlan) Summary() LogDisposalSummary {
	if p == nil {
		return LogDisposalSummary{}
	}
	return p.summary
}

// ValidateProductionLogDirectory accepts only the canonical production path
// and rejects symlinks before any files are inspected.
func ValidateProductionLogDirectory(path string) error {
	if strings.TrimSpace(path) == "" || filepath.Clean(path) != ProductionLogDirectory {
		return fmt.Errorf("log directory must be %s", ProductionLogDirectory)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect production log directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("production log directory must be a real directory")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || filepath.Clean(resolved) != ProductionLogDirectory {
		return fmt.Errorf("production log directory must not traverse symbolic links")
	}
	return nil
}

// AnalyzeLogDirectory inspects only first-level IAM app/warn/error log files.
func AnalyzeLogDirectory(path string) (*LogDisposalPlan, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect log directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("log directory must be a real directory")
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read log directory: %w", err)
	}
	plan := &LogDisposalPlan{}
	for _, entry := range entries {
		if !isIAMLogFile(entry.Name()) {
			continue
		}
		fullPath := filepath.Join(path, entry.Name())
		fileInfo, err := os.Lstat(fullPath)
		if err != nil {
			return nil, fmt.Errorf("inspect IAM log file: %w", err)
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("IAM log candidate must be a regular file")
		}
		refreshMatches, sqlMatches, err := countSensitiveLogMarkers(fullPath)
		if err != nil {
			return nil, err
		}
		plan.candidates = append(plan.candidates, logCandidate{path: fullPath, info: fileInfo})
		plan.summary.FileCount++
		plan.summary.TotalBytes += fileInfo.Size()
		plan.summary.RefreshTokenLogMatches += refreshMatches
		plan.summary.GORMSQLLogMatches += sqlMatches
		modifiedAt := fileInfo.ModTime()
		if plan.summary.EarliestModifiedAt == nil || modifiedAt.Before(*plan.summary.EarliestModifiedAt) {
			earliest := modifiedAt
			plan.summary.EarliestModifiedAt = &earliest
		}
		if plan.summary.LatestModifiedAt == nil || modifiedAt.After(*plan.summary.LatestModifiedAt) {
			latest := modifiedAt
			plan.summary.LatestModifiedAt = &latest
		}
	}
	return plan, nil
}

// Dispose removes exactly the first-level files selected during analysis.
func (p *LogDisposalPlan) Dispose() (LogDisposalSummary, error) {
	if p == nil {
		return LogDisposalSummary{}, fmt.Errorf("log disposal plan is required")
	}
	summary := p.summary
	for _, candidate := range p.candidates {
		info, err := os.Lstat(candidate.path)
		if err != nil {
			return summary, fmt.Errorf("revalidate IAM log file: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !os.SameFile(candidate.info, info) {
			return summary, fmt.Errorf("IAM log candidate changed before deletion")
		}
		if err := os.Remove(candidate.path); err != nil {
			return summary, fmt.Errorf("delete IAM log file: %w", err)
		}
		summary.DeletedFiles++
	}
	return summary, nil
}

func isIAMLogFile(name string) bool {
	for _, family := range []string{"app", "warn", "error"} {
		if name == family+".log" {
			return true
		}
		if strings.HasPrefix(name, family+".") &&
			(strings.HasSuffix(name, ".log") ||
				strings.Contains(name, ".log.") ||
				strings.HasSuffix(name, ".log.gz")) {
			return true
		}
	}
	return false
}

func countSensitiveLogMarkers(path string) (int64, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open IAM log file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var reader io.Reader = file
	if strings.HasSuffix(path, ".gz") {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return 0, 0, fmt.Errorf("read compressed IAM log file: %w", err)
		}
		defer func() { _ = gzipReader.Close() }()
		reader = gzipReader
	}

	var refreshMatches, sqlMatches int64
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 4*1024*1024)
	for scanner.Scan() {
		line := strings.ToLower(scanner.Text())
		if strings.Contains(line, "refresh token cached") ||
			strings.Contains(line, "refresh token deleted") ||
			strings.Contains(line, "token_hint") ||
			strings.Contains(line, "refresh_token:") {
			refreshMatches++
		}
		if strings.Contains(line, "gorm trace") ||
			strings.Contains(line, "gorm slow query") ||
			strings.Contains(line, `"sql":`) {
			sqlMatches++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("scan IAM log file: %w", err)
	}
	return refreshMatches, sqlMatches, nil
}

func MarshalLogDisposalSummary(summary LogDisposalSummary) ([]byte, error) {
	return json.Marshal(summary)
}
