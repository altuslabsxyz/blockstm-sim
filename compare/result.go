package compare

type Verdict string

const (
	Match      Verdict = "MATCH"
	Divergence Verdict = "DIVERGENCE"
)

type Result struct {
	Verdict       Verdict
	Height        int64
	Findings      []Finding
	MsgKeys       []string   // fixture msg keys exercised in this block, one per tx
	TxWriteSets   [][]string // oracle write keys per tx, parallel to MsgKeys; nil when unavailable
	OracleTxCodes []uint32   // error code per oracle tx; populated for all blocks

	// HotKeys aggregates the probe's BlockSTM validation conflicts by store/key.
	// nil when no conflict observer is wired (public build) or no conflicts occurred.
	// Performance diagnostics only — never affects Verdict.
	HotKeys []HotKeyStat
	// ExecutionRatio is the probe's executedTxns/blockSize (1.0 = no re-execution).
	// 0 when execution stats are unavailable. Never affects Verdict.
	ExecutionRatio float64
}
