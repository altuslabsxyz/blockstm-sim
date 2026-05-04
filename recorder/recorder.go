package recorder

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/coverage"
)

const SchemaVersion = 1

// RunRecord is the first NDJSON line — run-level metadata.
type RunRecord struct {
	SchemaVersion int       `json:"schema_version"`
	RunID         string    `json:"run_id"`
	StartedAt     time.Time `json:"started_at"`
	CorpusDir     string    `json:"corpus_dir"`
	SimVersion    string    `json:"sim_version"`
}

// BlockRecord is one NDJSON line per processed block.
type BlockRecord struct {
	Height      int64             `json:"height"`
	FixtureName string            `json:"fixture_name"`
	Verdict     string            `json:"verdict"`
	Divergences []DivergenceEntry `json:"divergences,omitempty"`
	MsgTypes    []coverage.Entry  `json:"msg_types,omitempty"`
}

// DivergenceEntry captures a single finding.
type DivergenceEntry struct {
	FindingID  string `json:"finding_id"`
	TxIndex    int    `json:"tx_index"`
	ProbeIndex int    `json:"probe_index"`
	Dimension  string `json:"dimension"`
	Oracle     string `json:"oracle"`
	Probe      string `json:"probe"`
}

// Sink records run results as NDJSON.
type Sink interface {
	WriteHeader(RunRecord) error
	WriteBlock(BlockRecord) error
	Close() error
}

// NDJSONSink writes run records to a .ndjson file.
type NDJSONSink struct {
	f      *os.File
	enc    *json.Encoder
	errLog *log.Logger
}

// New creates an NDJSONSink writing to dir/{runID}.ndjson.
// The directory is created if it does not exist.
func New(dir, runID string, errLog *log.Logger) (*NDJSONSink, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create record dir: %w", err)
	}

	path := filepath.Join(dir, runID+".ndjson")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create record file: %w", err)
	}

	return &NDJSONSink{
		f:      f,
		enc:    json.NewEncoder(f),
		errLog: errLog,
	}, nil
}

func (s *NDJSONSink) WriteHeader(r RunRecord) error {
	if err := s.enc.Encode(r); err != nil {
		return fmt.Errorf("write run header: %w", err)
	}
	return nil
}

func (s *NDJSONSink) WriteBlock(r BlockRecord) error {
	if err := s.enc.Encode(r); err != nil {
		s.errLog.Printf("write block record (height %d): %v", r.Height, err)
	}
	return nil
}

func (s *NDJSONSink) Close() error {
	syncErr := s.f.Sync()
	closeErr := s.f.Close()
	if syncErr != nil {
		return fmt.Errorf("sync record file: %w", syncErr)
	}
	return closeErr
}

// NopSink discards all writes. Used when recording is disabled.
type NopSink struct{}

func (NopSink) WriteHeader(RunRecord) error { return nil }
func (NopSink) WriteBlock(BlockRecord) error { return nil }
func (NopSink) Close() error                 { return nil }

// GenerateRunID returns a timestamp-based ID: YYYYMMDD-HHmmss-XXXX.
func GenerateRunID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

// BlockRecordFromResult maps a compare.Result to a BlockRecord.
func BlockRecordFromResult(r *compare.Result, fixtureName string) BlockRecord {
	br := BlockRecord{
		Height:      r.Height,
		FixtureName: fixtureName,
		Verdict:     string(r.Verdict),
	}
	for _, f := range r.Findings {
		br.Divergences = append(br.Divergences, DivergenceEntry{
			FindingID:  f.ID,
			TxIndex:    f.TxIndex,
			ProbeIndex: f.ProbeIndex,
			Dimension:  string(f.Dimension),
			Oracle:     f.Oracle,
			Probe:      f.Probe,
		})
	}
	if len(r.MsgKeys) > 0 {
		reg := coverage.Registered()
		seen := make(map[string]struct{}, len(r.MsgKeys))
		var entries []coverage.Entry
		for _, k := range r.MsgKeys {
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			if e, ok := reg[k]; ok {
				entries = append(entries, e)
			} else {
				entries = append(entries, coverage.Entry{Key: k})
			}
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
		br.MsgTypes = entries
	}
	return br
}
