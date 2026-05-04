package run

import "github.com/altuslabsxyz/blockstm-sim/coverage"

func init() {
	coverage.Register("bank-send", coverage.Entry{
		Key:       "bank-send",
		Module:    "bank",
		MsgType:   "MsgSend",
		HandlerFn: "Send",
	})
}
