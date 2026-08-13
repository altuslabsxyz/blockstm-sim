package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

// MarkdownReporter collects run output and emits Markdown at Footer.
type MarkdownReporter struct {
	out    io.Writer
	corpus string
	probes int
	blocks []BlockOutcome
}

// NewMarkdown returns a MarkdownReporter writing to out.
func NewMarkdown(out io.Writer) *MarkdownReporter {
	return &MarkdownReporter{out: out}
}

func (r *MarkdownReporter) Errors() int { return 0 }

func (r *MarkdownReporter) Header(corpus string, _, probes int) {
	r.corpus = corpus
	r.probes = probes
}

func (r *MarkdownReporter) Block(o BlockOutcome) {
	r.blocks = append(r.blocks, o)
}

func (r *MarkdownReporter) Footer(s Summary, failOnDivergence bool) {
	r.write("# BlockSTM Sim — Run Report\n\n")
	r.write("**Corpus:** %s  **Probes:** %d\n\n", r.corpus, r.probes)

	r.write("## Summary\n\n")
	r.write("| Metric | Count |\n|--------|-------|\n")
	r.write("| Total Blocks | %d |\n", s.TotalBlocks)
	r.write("| OK | %d |\n", s.OKCount)
	r.write("| Divergences | %d |\n", s.DivergenceCount)
	r.write("| Canary Expected | %d |\n", s.CanaryExpected)
	r.write("| Canary Missed | %d |\n", s.CanaryMissed)
	if s.MaxExecRatio > 0 {
		r.write("| Max Execution Ratio | %.2f |\n", s.MaxExecRatio)
	}
	r.write("\n")

	var divBlocks, missedCanaries, perfBlocks []BlockOutcome
	for _, b := range r.blocks {
		switch {
		case b.IsCanary && b.Verdict == compare.Match:
			missedCanaries = append(missedCanaries, b)
		case b.Verdict == compare.Divergence:
			divBlocks = append(divBlocks, b)
		}
		if len(b.HotKeys) > 0 {
			perfBlocks = append(perfBlocks, b)
		}
	}

	if len(divBlocks) > 0 {
		r.write("## Divergences\n\n")
		for _, b := range divBlocks {
			r.write("### %s\n\n", b.FixtureName)
			r.write("| ID | Height | Tx | Probe | Dimension |\n|----|--------|----|-------|-----------|\n")
			for _, f := range b.Findings {
				r.write("| `%s` | %d | %d | %d | %s |\n",
					f.ID, f.Height, f.TxIndex, f.ProbeIndex, f.Dimension)
			}
			r.write("\n")
		}
	}

	if len(missedCanaries) > 0 {
		r.write("## Missed Canaries\n\n")
		for _, b := range missedCanaries {
			r.write("- %s\n", b.FixtureName)
		}
		r.write("\n")
	}

	if len(perfBlocks) > 0 {
		r.write("## Hot Conflict Keys\n\n")
		r.write("Max execution ratio: **%s** — blocks with hot keys: %d\n\n",
			formatRatio(s.MaxExecRatio), s.HotKeyBlocks)
		for _, b := range perfBlocks {
			r.write("### %s\n\n", b.FixtureName)
			if b.ExecutionRatio > 0 {
				r.write("Execution ratio: **%.2f**\n\n", b.ExecutionRatio)
			}
			r.write("| Store | Key | Conflicts | Txs |\n|-------|-----|-----------|-----|\n")
			for _, hk := range b.HotKeys {
				r.write("| %s | `%s` | %d | %d |\n", hk.Store, hk.Key, hk.Conflicts, len(hk.Txs))
			}
			r.write("\n")
		}
	}

	repro := reproRunCommand(r.corpus, r.probes, failOnDivergence)
	r.write("## Reproduce\n\n```sh\n%s\n```\n", repro)
}

func (r *MarkdownReporter) write(format string, args ...any) {
	_, _ = fmt.Fprintf(r.out, format, args...)
}

func reproRunCommand(corpus string, probes int, failOnDiv bool) string {
	var sb strings.Builder
	sb.WriteString("blockstm-sim run")
	if corpus != "fixtures" {
		_, _ = fmt.Fprintf(&sb, " --corpus %s", corpus)
	}
	if probes != 1 {
		_, _ = fmt.Fprintf(&sb, " --probes %d", probes)
	}
	if failOnDiv {
		sb.WriteString(" --fail-on-divergence")
	}
	return sb.String()
}
