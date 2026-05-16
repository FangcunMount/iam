package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

const snapshotFileName = "snapshot.txt"

// FileSnapshotWriter persists profile suggestion candidates in a line format.
type FileSnapshotWriter struct {
	dataDir string
}

// NewFileSnapshotWriter creates a snapshot writer.
func NewFileSnapshotWriter(dataDir string) *FileSnapshotWriter {
	return &FileSnapshotWriter{dataDir: strings.TrimSpace(dataDir)}
}

// Write persists terms as name|id|mobiles|tenant|org|owners|weight lines.
func (w *FileSnapshotWriter) Write(_ context.Context, terms []domainsuggest.ProfileSearchTerm) error {
	if w == nil || w.dataDir == "" || len(terms) == 0 {
		return nil
	}
	if err := os.MkdirAll(w.dataDir, 0o755); err != nil {
		return err
	}
	lines := make([]string, 0, len(terms))
	for _, term := range terms {
		lines = append(lines, EncodeSnapshotLine(term))
	}
	return os.WriteFile(filepath.Join(w.dataDir, snapshotFileName), []byte(strings.Join(lines, "\n")), 0o644)
}

// EncodeSnapshotLine serializes a ProfileSearchTerm.
func EncodeSnapshotLine(term domainsuggest.ProfileSearchTerm) string {
	owners := int64SliceToCSV(term.OwnerOperatorIDs)
	return fmt.Sprintf("%s|%d|%s|%d|%d|%s|%d",
		strings.TrimSpace(term.DisplayName),
		term.ProfileID,
		strings.Join(term.Mobiles, ","),
		term.TenantID,
		term.OrgID,
		owners,
		term.Weight,
	)
}

func int64SliceToCSV(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

// DecodeSnapshotLine parses snapshot rows（兼容旧版 name|id|mobiles|-|weight）。
func DecodeSnapshotLine(line string) (domainsuggest.ProfileSearchTerm, bool) {
	parts := strings.Split(line, "|")
	if len(parts) < 5 {
		return domainsuggest.ProfileSearchTerm{}, false
	}
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return domainsuggest.ProfileSearchTerm{}, false
	}
	id, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	weight, _ := strconv.Atoi(strings.TrimSpace(parts[len(parts)-1]))

	if len(parts) >= 7 {
		tenant, _ := strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
		org, _ := strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64)
		owners := splitInt64CSV(strings.TrimSpace(parts[5]))
		return domainsuggest.NewProfileSearchTerm(
			id,
			name,
			splitMobiles(strings.TrimSpace(parts[2])),
			weight,
			tenant,
			org,
			owners,
		), true
	}

	// legacy: name|id|mobiles|placeholder|weight
	return domainsuggest.NewProfileSearchTerm(
		id,
		name,
		splitMobiles(strings.TrimSpace(parts[2])),
		weight,
		0,
		0,
		nil,
	), true
}

// TermsFromSnapshotLines converts snapshot rows into profile search terms.
func TermsFromSnapshotLines(lines []string) []domainsuggest.ProfileSearchTerm {
	out := make([]domainsuggest.ProfileSearchTerm, 0, len(lines))
	for _, line := range lines {
		term, ok := DecodeSnapshotLine(line)
		if !ok {
			continue
		}
		out = append(out, term)
	}
	return out
}

func splitMobiles(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	mobiles := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		mobiles = append(mobiles, part)
	}
	return mobiles
}

func splitInt64CSV(value string) []int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil || n == 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}
