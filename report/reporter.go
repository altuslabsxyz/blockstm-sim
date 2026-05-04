package report

// Reporter abstracts run-command output format.
type Reporter interface {
	Header(corpus string, blocks, probes int)
	Block(o BlockOutcome)
	Footer(s Summary, failOnDivergence bool)
	Errors() int
}
