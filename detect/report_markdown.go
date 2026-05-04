package detect

import (
	"fmt"
	"io"
	"sort"
)

// MarkdownReporter buffers detect findings and emits Markdown at Footer, grouped by module.
type MarkdownReporter struct {
	out      io.Writer
	sdkPath  string
	findings []Finding
}

// NewMarkdownReporter returns a MarkdownReporter writing to out.
func NewMarkdownReporter(out io.Writer) *MarkdownReporter {
	return &MarkdownReporter{out: out}
}

func (r *MarkdownReporter) Header(sdkPath string) {
	r.sdkPath = sdkPath
}

func (r *MarkdownReporter) Finding(f Finding) {
	r.findings = append(r.findings, f)
}

func (r *MarkdownReporter) Footer(result *ScanResult, _ string) {
	w := r.out
	fmt.Fprintf(w, "# BlockSTM Sim — Detect Report\n\n")
	fmt.Fprintf(w, "**SDK Path:** %s\n\n", r.sdkPath)

	var timeCnt, randCnt, ioCnt int
	for _, f := range r.findings {
		switch f.Category {
		case CatTime:
			timeCnt++
		case CatRand:
			randCnt++
		case CatIO:
			ioCnt++
		}
	}

	fmt.Fprintf(w, "## Summary\n\n")
	fmt.Fprintf(w, "| Category | Count |\n|----------|-------|\n")
	fmt.Fprintf(w, "| time | %d |\n", timeCnt)
	fmt.Fprintf(w, "| rand | %d |\n", randCnt)
	fmt.Fprintf(w, "| io | %d |\n", ioCnt)
	fmt.Fprintf(w, "| **Total** | **%d** |\n\n", len(r.findings))
	fmt.Fprintf(w, "Scanned %d files.\n\n", result.Files)

	byModule := make(map[string][]Finding)
	var modules []string
	for _, f := range r.findings {
		if _, ok := byModule[f.Module]; !ok {
			modules = append(modules, f.Module)
		}
		byModule[f.Module] = append(byModule[f.Module], f)
	}
	sort.Strings(modules)

	if len(modules) > 0 {
		fmt.Fprintf(w, "## Findings by Module\n\n")
		for _, mod := range modules {
			fmt.Fprintf(w, "### %s\n\n", mod)
			fmt.Fprintf(w, "| File | Line | Function | Call | Category |\n|------|------|----------|------|----------|\n")
			for _, f := range byModule[mod] {
				fmt.Fprintf(w, "| `%s` | %d | `%s` | `%s` | %s |\n",
					f.File, f.Line, f.FuncName, f.Call, f.Category)
			}
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintf(w, "## Reproduce\n\n```sh\nblockstm-sim detect --sdk-path %s\n```\n", r.sdkPath)
}
