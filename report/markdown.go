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
	w := r.out
	fmt.Fprintf(w, "# BlockSTM Sim — Run Report\n\n")
	fmt.Fprintf(w, "**Corpus:** %s  **Probes:** %d\n\n", r.corpus, r.probes)

	fmt.Fprintf(w, "## Summary\n\n")
	fmt.Fprintf(w, "| Metric | Count |\n|--------|-------|\n")
	fmt.Fprintf(w, "| Total Blocks | %d |\n", s.TotalBlocks)
	fmt.Fprintf(w, "| OK | %d |\n", s.OKCount)
	fmt.Fprintf(w, "| Divergences | %d |\n", s.DivergenceCount)
	fmt.Fprintf(w, "| Canary Expected | %d |\n", s.CanaryExpected)
	fmt.Fprintf(w, "| Canary Missed | %d |\n\n", s.CanaryMissed)

	var divBlocks, missedCanaries []BlockOutcome
	for _, b := range r.blocks {
		switch {
		case b.IsCanary && b.Verdict == compare.Match:
			missedCanaries = append(missedCanaries, b)
		case b.Verdict == compare.Divergence:
			divBlocks = append(divBlocks, b)
		}
	}

	if len(divBlocks) > 0 {
		fmt.Fprintf(w, "## Divergences\n\n")
		for _, b := range divBlocks {
			fmt.Fprintf(w, "### %s\n\n", b.FixtureName)
			fmt.Fprintf(w, "| ID | Height | Tx | Probe | Dimension |\n|----|--------|----|-------|-----------|\n")
			for _, f := range b.Findings {
				fmt.Fprintf(w, "| `%s` | %d | %d | %d | %s |\n",
					f.ID, f.Height, f.TxIndex, f.ProbeIndex, f.Dimension)
			}
			fmt.Fprintln(w)
		}
	}

	if len(missedCanaries) > 0 {
		fmt.Fprintf(w, "## Missed Canaries\n\n")
		for _, b := range missedCanaries {
			fmt.Fprintf(w, "- %s\n", b.FixtureName)
		}
		fmt.Fprintln(w)
	}

	repro := reproRunCommand(r.corpus, r.probes, failOnDivergence)
	fmt.Fprintf(w, "## Reproduce\n\n```sh\n%s\n```\n", repro)
}

func reproRunCommand(corpus string, probes int, failOnDiv bool) string {
	var sb strings.Builder
	sb.WriteString("blockstm-sim run")
	if corpus != "fixtures" {
		fmt.Fprintf(&sb, " --corpus %s", corpus)
	}
	if probes != 1 {
		fmt.Fprintf(&sb, " --probes %d", probes)
	}
	if failOnDiv {
		sb.WriteString(" --fail-on-divergence")
	}
	return sb.String()
}
