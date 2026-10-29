package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
)

// Before and proposed After are fsynced before COMMIT. A lost ACK is classified
// by exact readback, never retried as another mutation. The original record is
// retained even when readback proves the write reached the server.
type journalEntry struct {
	Sequence    int        `json:"sequence"`
	PreviousSHA string     `json:"previous_sha256"`
	Operation   string     `json:"operation_id"`
	ArchiveSHA  string     `json:"archive_sha256"`
	Phase       string     `json:"phase"`
	Before      tableImage `json:"before"`
	After       tableImage `json:"after"`
	Result      string     `json:"result"`
}
type journal struct {
	f                     *os.File
	seq                   int
	previous              string
	operation, archiveSHA string
}

func newJournal(path, operation, archiveSHA string) (*journal, error) {
	if e := writeExclusive(path, nil); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if e != nil {
		return nil, e
	}
	return &journal{f: f, operation: operation, archiveSHA: archiveSHA}, nil
}
func (j *journal) append(phase string, before, after tableImage, result string) error {
	j.seq++
	v := journalEntry{j.seq, j.previous, j.operation, j.archiveSHA, phase, before, after, result}
	raw := encode(v)
	n, e := j.f.Write(raw)
	if e != nil {
		return e
	}
	if n != len(raw) {
		return errors.New("short journal write")
	}
	if e = j.f.Sync(); e != nil {
		return e
	}
	j.previous = rawSHA(raw)
	return nil
}
func parseJournal(raw []byte, operation, archiveSHA string) ([]journalEntry, error) {
	out := []journalEntry{}
	s := bufio.NewScanner(bytes.NewReader(raw))
	s.Buffer(make([]byte, 65536), 64<<20)
	prev := ""
	for s.Scan() {
		line := append(append([]byte{}, s.Bytes()...), '\n')
		var e journalEntry
		if err := strictJSON(s.Bytes(), &e); err != nil {
			return nil, err
		}
		if e.Sequence != len(out)+1 || e.PreviousSHA != prev || e.Operation != operation || e.ArchiveSHA != archiveSHA {
			return nil, errors.New("journal chain binding rejected")
		}
		switch e.Result {
		case "prepared", "committed", "readback_before", "readback_after", "readback_mixed", "sessions_revoked", "complete":
		default:
			return nil, fmt.Errorf("unknown journal result")
		}
		out = append(out, e)
		prev = rawSHA(line)
	}
	if e := s.Err(); e != nil {
		return nil, e
	}
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		return nil, errors.New("partial journal retained; mutation forbidden")
	}
	return out, nil
}
func reconcileRows(current, before, after []row) string {
	if rowsEqual(current, before) {
		return "readback_before"
	}
	if rowsEqual(current, after) {
		return "readback_after"
	}
	return "readback_mixed"
}

// Known postimages are eligible only when they came from this archive's
// fsynced, hash-chained journal. No arbitrary caller-provided row is accepted.
func knownPostimages(entries []journalEntry) map[string][]row {
	out := map[string][]row{}
	for _, e := range entries {
		if e.Result != "prepared" {
			continue
		}
		for _, r := range e.After.Rows {
			k := e.After.Name + ":" + string(field(e.After, r, "id").Raw)
			out[k] = append(out[k], r)
		}
	}
	return out
}
