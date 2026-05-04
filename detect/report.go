package detect

import (
	"fmt"
	"io"
)

type Reporter struct {
	out io.Writer
}

func NewReporter(out io.Writer) *Reporter {
	return &Reporter{out: out}
}

func (r *Reporter) Header(sdkPath string) {
	r.write("Detect  sdk-path=%s\n\n", sdkPath)
}

func (r *Reporter) Finding(f Finding) {
	r.write("[%s] %s:%d  %s\n", f.Category, f.File, f.Line, f.FuncName)
	r.write("       %s\n", f.Call)
}

func (r *Reporter) Footer(result *ScanResult, sdkPath string) {
	var timeCnt, randCnt, ioCnt int
	for _, f := range result.Findings {
		switch f.Category {
		case CatTime:
			timeCnt++
		case CatRand:
			randCnt++
		case CatIO:
			ioCnt++
		}
	}
	total := len(result.Findings)
	r.write("\nSummary\n  %d findings / %d time / %d rand / %d io\n  Scanned %d files in %s\n",
		total, timeCnt, randCnt, ioCnt, result.Files, sdkPath)
}

func (r *Reporter) write(format string, args ...any) {
	_, _ = fmt.Fprintf(r.out, format, args...)
}
