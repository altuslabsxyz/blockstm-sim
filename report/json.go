package report

import (
	"encoding/json"
	"fmt"
	"io"
)

const jsonSchemaVersion = 1

type jsonFinding struct {
	ID         string `json:"id"`
	Height     int64  `json:"height"`
	TxIndex    int    `json:"tx_index"`
	ProbeIndex int    `json:"probe_index"`
	Dimension  string `json:"dimension"`
	Oracle     string `json:"oracle"`
	Probe      string `json:"probe"`
}

type jsonBlock struct {
	Index    int           `json:"index"`
	Fixture  string        `json:"fixture"`
	IsCanary bool          `json:"is_canary"`
	Verdict  string        `json:"verdict"`
	Findings []jsonFinding `json:"findings,omitempty"`
}

type jsonRunSummary struct {
	TotalBlocks    int `json:"total_blocks"`
	OK             int `json:"ok"`
	Divergences    int `json:"divergences"`
	CanaryExpected int `json:"canary_expected"`
	CanaryMissed   int `json:"canary_missed"`
	ExitCode       int `json:"exit_code"`
}

type jsonRunReport struct {
	SchemaVersion int            `json:"schema_version"`
	Corpus        string         `json:"corpus"`
	Probes        int            `json:"probes"`
	Summary       jsonRunSummary `json:"summary"`
	Blocks        []jsonBlock    `json:"blocks"`
}

// JSONReporter collects run output and emits a single JSON document at Footer.
type JSONReporter struct {
	out    io.Writer
	corpus string
	probes int
	blocks []jsonBlock
}

// NewJSON returns a JSONReporter writing to out.
func NewJSON(out io.Writer) *JSONReporter {
	return &JSONReporter{out: out}
}

func (r *JSONReporter) Errors() int { return 0 }

func (r *JSONReporter) Header(corpus string, _, probes int) {
	r.corpus = corpus
	r.probes = probes
}

func (r *JSONReporter) Block(o BlockOutcome) {
	jb := jsonBlock{
		Index:    o.Index,
		Fixture:  o.FixtureName,
		IsCanary: o.IsCanary,
		Verdict:  string(o.Verdict),
	}
	for _, f := range o.Findings {
		jb.Findings = append(jb.Findings, jsonFinding{
			ID:         f.ID,
			Height:     f.Height,
			TxIndex:    f.TxIndex,
			ProbeIndex: f.ProbeIndex,
			Dimension:  string(f.Dimension),
			Oracle:     f.Oracle,
			Probe:      f.Probe,
		})
	}
	r.blocks = append(r.blocks, jb)
}

func (r *JSONReporter) Footer(s Summary, failOnDivergence bool) {
	doc := jsonRunReport{
		SchemaVersion: jsonSchemaVersion,
		Corpus:        r.corpus,
		Probes:        r.probes,
		Summary: jsonRunSummary{
			TotalBlocks:    s.TotalBlocks,
			OK:             s.OKCount,
			Divergences:    s.DivergenceCount,
			CanaryExpected: s.CanaryExpected,
			CanaryMissed:   s.CanaryMissed,
			ExitCode:       s.ExitCode(failOnDivergence),
		},
		Blocks: r.blocks,
	}
	enc := json.NewEncoder(r.out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		_, _ = fmt.Fprintf(r.out, "{\"error\":%q}\n", err.Error())
	}
}
