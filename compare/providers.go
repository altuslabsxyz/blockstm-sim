package compare

// WriteSetProvider returns the sorted write keys for a given transaction.
type WriteSetProvider interface {
	TxWriteSet(txIndex int) []string
}
