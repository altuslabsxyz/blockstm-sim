package report

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/coverage"
)

type BlockOutcome struct {
	Index          int
	Total          int
	FixtureName    string
	IsCanary       bool
	Verdict        compare.Verdict
	Findings       []compare.Finding
	// OracleTxCodes holds the error code for each oracle tx result.
	// Populated only for canary blocks so the reporter can surface
	// ante handler rejections on CANARY MISSED without impacting normal output.
	OracleTxCodes  []uint32
}

type Summary struct {
	Corpus          string
	TotalBlocks     int
	OKCount         int
	DivergenceCount int
	CanaryExpected  int
	CanaryMissed    int
	ReporterErrors  int
	Coverage        coverage.Report
	StatePatterns   coverage.StatePatternReport
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
		for i, code := range o.OracleTxCodes {
			if code != 0 {
				r.write(fmt.Sprintf("  oracle tx[%d]: code=%d\n", i, code))
			}
		}
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
	r.write(fmt.Sprintf("\nSummary\n  %d blocks run / %d ok / %d divergence (%d canary expected) / %d canary missed\n",
		s.TotalBlocks, s.OKCount, s.DivergenceCount, s.CanaryExpected, s.CanaryMissed))
	r.writeCoverage(s.Coverage, s.StatePatterns)
	r.write(fmt.Sprintf("Exit: %d\n", exit))
}

func (r *CLIReporter) writeCoverage(cov coverage.Report, patterns coverage.StatePatternReport) {
	total := len(cov.Covered) + len(cov.Uncovered)
	if total == 0 {
		return
	}
	r.write(fmt.Sprintf("Coverage  registered=%d  covered=%d  uncovered=%d\n",
		total, len(cov.Covered), len(cov.Uncovered)))
	for _, s := range cov.Covered {
		patternSuffix := ""
		if n := patterns.DistinctCount(s.Key); n > 0 {
			patternSuffix = fmt.Sprintf("  %d state patterns", n)
		}
		r.write(fmt.Sprintf("  %-14s %-26s %-16s %d tx%s\n", s.Module, s.MsgType, s.HandlerFn, s.Count, patternSuffix))
	}
	for _, e := range cov.Uncovered {
		r.write(fmt.Sprintf("! %-14s %-26s %-16s 0 tx\n", e.Module, e.MsgType, e.HandlerFn))
	}
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
		_, _ = fmt.Fprintf(r.errw, "reporter: write error: %v\n", err)
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
