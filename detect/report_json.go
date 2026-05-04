package detect

import (
	"encoding/json"
	"io"
)

const jsonSchemaVersion = 1

type jsonDetectFinding struct {
	Category string `json:"category"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	FuncName string `json:"func_name"`
	Call     string `json:"call"`
	Module   string `json:"module"`
}

type jsonDetectSummary struct {
	Total        int `json:"total"`
	Time         int `json:"time"`
	Rand         int `json:"rand"`
	IO           int `json:"io"`
	FilesScanned int `json:"files_scanned"`
}

type jsonDetectReport struct {
	SchemaVersion int                 `json:"schema_version"`
	SDKPath       string              `json:"sdk_path"`
	Summary       jsonDetectSummary   `json:"summary"`
	Findings      []jsonDetectFinding `json:"findings"`
}

// JSONReporter buffers detect findings and emits JSON at Footer.
type JSONReporter struct {
	out      io.Writer
	sdkPath  string
	findings []Finding
}

// NewJSONReporter returns a JSONReporter writing to out.
func NewJSONReporter(out io.Writer) *JSONReporter {
	return &JSONReporter{out: out}
}

func (r *JSONReporter) Header(sdkPath string) {
	r.sdkPath = sdkPath
}

func (r *JSONReporter) Finding(f Finding) {
	r.findings = append(r.findings, f)
}

func (r *JSONReporter) Footer(result *ScanResult, _ string) {
	var timeCnt, randCnt, ioCnt int
	jf := make([]jsonDetectFinding, 0, len(r.findings))
	for _, f := range r.findings {
		switch f.Category {
		case CatTime:
			timeCnt++
		case CatRand:
			randCnt++
		case CatIO:
			ioCnt++
		}
		jf = append(jf, jsonDetectFinding{
			Category: string(f.Category),
			File:     f.File,
			Line:     f.Line,
			FuncName: f.FuncName,
			Call:     f.Call,
			Module:   f.Module,
		})
	}
	doc := jsonDetectReport{
		SchemaVersion: jsonSchemaVersion,
		SDKPath:       r.sdkPath,
		Summary: jsonDetectSummary{
			Total:        len(r.findings),
			Time:         timeCnt,
			Rand:         randCnt,
			IO:           ioCnt,
			FilesScanned: result.Files,
		},
		Findings: jf,
	}
	enc := json.NewEncoder(r.out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(doc)
}
