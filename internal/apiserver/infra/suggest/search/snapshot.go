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

// FileSnapshotWriter persists profile suggestion candidates in the legacy line format.
type FileSnapshotWriter struct {
	dataDir string
}

// NewFileSnapshotWriter creates a legacy-compatible snapshot writer.
func NewFileSnapshotWriter(dataDir string) *FileSnapshotWriter {
	return &FileSnapshotWriter{dataDir: strings.TrimSpace(dataDir)}
}

// Write persists candidates as name|id|mobiles|-|weight lines.
func (w *FileSnapshotWriter) Write(_ context.Context, candidates []domainsuggest.ProfileCandidate) error {
	if w == nil || w.dataDir == "" || len(candidates) == 0 {
		return nil
	}
	if err := os.MkdirAll(w.dataDir, 0o755); err != nil {
		return err
	}
	lines := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		lines = append(lines, EncodeLegacyLine(candidate))
	}
	return os.WriteFile(filepath.Join(w.dataDir, snapshotFileName), []byte(strings.Join(lines, "\n")), 0o644)
}

// EncodeLegacyLine keeps the old snapshot format as an infrastructure compatibility detail.
func EncodeLegacyLine(candidate domainsuggest.ProfileCandidate) string {
	return fmt.Sprintf("%s|%d|%s|-|%d",
		strings.TrimSpace(candidate.DisplayName),
		candidate.ProfileID,
		strings.Join(candidate.Mobiles, ","),
		candidate.Weight,
	)
}

// DecodeLegacyLine parses name|id|mobiles|disease|weight snapshot rows.
func DecodeLegacyLine(line string) (domainsuggest.ProfileCandidate, bool) {
	parts := strings.Split(line, "|")
	if len(parts) < 5 {
		return domainsuggest.ProfileCandidate{}, false
	}
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return domainsuggest.ProfileCandidate{}, false
	}
	id, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	weight, _ := strconv.Atoi(strings.TrimSpace(parts[4]))
	return domainsuggest.NewProfileCandidate(
		id,
		name,
		splitMobiles(strings.TrimSpace(parts[2])),
		weight,
	), true
}

// CandidatesFromLegacyLines converts snapshot rows into profile candidates.
func CandidatesFromLegacyLines(lines []string) []domainsuggest.ProfileCandidate {
	candidates := make([]domainsuggest.ProfileCandidate, 0, len(lines))
	for _, line := range lines {
		candidate, ok := DecodeLegacyLine(line)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
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
