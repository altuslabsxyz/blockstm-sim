package compare

type Verdict string

const (
	Match      Verdict = "MATCH"
	Divergence Verdict = "DIVERGENCE"
)

type Result struct {
	Verdict  Verdict
	Height   int64
	Findings []Finding
}
