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

	r.write("# BlockSTM Sim — Detect Report\n\n")
	r.write("**SDK Path:** %s\n\n", r.sdkPath)

	r.write("## Summary\n\n")
	r.write("| Category | Count |\n|----------|-------|\n")
	r.write("| time | %d |\n", timeCnt)
	r.write("| rand | %d |\n", randCnt)
	r.write("| io | %d |\n", ioCnt)
	r.write("| **Total** | **%d** |\n\n", len(r.findings))
	r.write("Scanned %d files.\n\n", result.Files)

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
		r.write("## Findings by Module\n\n")
		for _, mod := range modules {
			r.write("### %s\n\n", mod)
			r.write("| File | Line | Function | Call | Category |\n|------|------|----------|------|----------|\n")
			for _, f := range byModule[mod] {
				r.write("| `%s` | %d | `%s` | `%s` | %s |\n",
					f.File, f.Line, f.FuncName, f.Call, f.Category)
			}
			r.write("\n")
		}
	}

	r.write("## Reproduce\n\n```sh\nblockstm-sim detect --sdk-path %s\n```\n", r.sdkPath)
}

func (r *MarkdownReporter) write(format string, args ...any) {
	_, _ = fmt.Fprintf(r.out, format, args...)
}
