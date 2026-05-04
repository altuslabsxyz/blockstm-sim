package report

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

type BlockOutcome struct {
	Index       int
	Total       int
	FixtureName string
	IsCanary    bool
	Verdict     compare.Verdict
	Findings    []compare.Finding
}

type Summary struct {
	Corpus          string
	TotalBlocks     int
	OKCount         int
	DivergenceCount int
	CanaryExpected  int
	CanaryMissed    int
	ReporterErrors  int
}

func (s Summary) ExitCode(failOnDivergence bool) int {
	if s.CanaryMissed > 0 {
		return 1
	}
	if failOnDivergence && s.DivergenceCount > 0 {
		return 1
	}
	return 0
}

type CLIReporter struct {
	out  io.Writer
	errw io.Writer
	errs int
}

func NewCLI(out, errOut io.Writer) *CLIReporter {
	return &CLIReporter{out: out, errw: errOut}
}

func (r *CLIReporter) Errors() int { return r.errs }

func (r *CLIReporter) Header(corpus string, blocks, probes int) {
	r.write(fmt.Sprintf("Harness  corpus=%s  blocks=%d  probes=%d\n\n", corpus, blocks, probes))
}

func (r *CLIReporter) Block(o BlockOutcome) {
	w := digitWidth(o.Total)
	prefix := fmt.Sprintf("[%*d/%d]", w, o.Index, o.Total)

	switch {
	case o.IsCanary && o.Verdict == compare.Match:
		r.write(fmt.Sprintf("%s CANARY MISSED %s\n", prefix, o.FixtureName))
	case o.Verdict == compare.Match:
		r.write(fmt.Sprintf("%s ok %s\n", prefix, o.FixtureName))
	case o.Verdict == compare.Divergence:
		r.write(fmt.Sprintf("%s DIVERGENCE %s\n", prefix, o.FixtureName))
		for _, f := range o.Findings {
			r.writeFinding(f)
		}
	}
}

func (r *CLIReporter) Footer(s Summary, failOnDivergence bool) {
	exit := s.ExitCode(failOnDivergence)
	r.write(fmt.Sprintf("\nSummary\n  %d blocks run / %d ok / %d divergence (%d canary expected) / %d canary missed\nExit: %d\n",
		s.TotalBlocks, s.OKCount, s.DivergenceCount, s.CanaryExpected, s.CanaryMissed, exit))
}

func (r *CLIReporter) writeFinding(f compare.Finding) {
	pad := "             "
	r.write(fmt.Sprintf("%sdimension  : %s\n", pad, f.Dimension))
	r.write(fmt.Sprintf("%soracle     : %s\n", pad, truncHash(f.Oracle)))
	r.write(fmt.Sprintf("%scandidate %d: %s\n", pad, f.ProbeIndex, truncHash(f.Probe)))
	r.write(fmt.Sprintf("%sdetails    : %s\n", pad, detailsFor(f.Dimension)))
}

func (r *CLIReporter) write(s string) {
	if _, err := io.WriteString(r.out, s); err != nil {
		r.errs++
		fmt.Fprintf(r.errw, "reporter: write error: %v\n", err)
	}
}

func truncHash(hex string) string {
	hex = strings.TrimPrefix(hex, "0x")
	if len(hex) <= 10 {
		return "0x" + hex
	}
	return "0x" + hex[:10] + "..."
}

func detailsFor(dim compare.Dimension) string {
	switch dim {
	case compare.DimAppHash:
		return "post-block app hash mismatch"
	case compare.DimErrorCode:
		return "per-tx error code mismatch"
	case compare.DimWriteSet:
		return "per-tx write set mismatch"
	default:
		return string(dim) + " mismatch"
	}
}

func digitWidth(n int) int {
	if n <= 0 {
		return 1
	}
	return int(math.Log10(float64(n))) + 1
}
