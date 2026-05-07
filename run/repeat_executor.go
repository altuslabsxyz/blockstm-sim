//go:build sdk_hooks

package run

import (
	"bytes"
	"encoding/hex"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/client"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/instrument"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// RepeatRunExecutor implements Executor for F2 repeat-run determinism checking.
// It runs 1 oracle app (sequential) + n probe apps (each with distinct scheduler
// perturbation) and compares all probes against each other.
type RepeatRunExecutor struct {
	n           int
	oracle      sdkhook.App
	probes      []sdkhook.App
	txConfig    client.TxConfig
	keys        map[string]cryptotypes.PrivKey
	accountNums map[string]uint64
	sequences   map[string]uint64
}

// NewRepeatRunExecutor creates a RepeatRunExecutor that will spin up n probe apps.
// n is clamped to a minimum of 1.
func NewRepeatRunExecutor(n int) *RepeatRunExecutor {
	if n < 1 {
		n = 1
	}
	return &RepeatRunExecutor{n: n}
}

// Init creates 1 oracle app (sequential) + n probe apps with distinct scheduler
// perturbations, all sharing the same genesis.
func (e *RepeatRunExecutor) Init(genesis compare.GenesisSpec) error {
	gs, err := initGenesis(genesis)
	if err != nil {
		return err
	}

	var txCfg client.TxConfig

	cfg := buildAppConfig()
	oracleApp, err := initApp(cfg, gs.baseCfg, &txCfg)
	if err != nil {
		return fmt.Errorf("setup oracle app: %w", err)
	}
	instrument.InstrumentApp(oracleApp, instrument.Options{Runner: instrument.RunnerSequential})

	// e.n holds the desired probe count set by NewRepeatRunExecutor; it is only
	// used here in Init to size the probe slice.
	probeCfgs := generateProbeConfigs(e.n)
	probes := make([]sdkhook.App, len(probeCfgs))
	for i, pcfg := range probeCfgs {
		probeApp, err := initApp(cfg, gs.baseCfg, nil)
		if err != nil {
			return fmt.Errorf("setup probe app %d: %w", i, err)
		}
		instrument.InstrumentSTM(probeApp, newSTMRunnerForProbe(pcfg, txCfg.TxDecoder(), probeApp.GetStoreKeys()))
		probes[i] = probeApp
	}

	e.oracle = oracleApp
	e.probes = probes
	e.txConfig = txCfg
	e.keys = gs.keys
	e.accountNums = gs.accountNums
	e.sequences = gs.sequences

	return nil
}

// RunBlock executes one block across the oracle + all n probes and returns a
// merged Result. F1 findings come from oracle vs probe[0]; F2 findings come
// from probe[0] vs probe[i] for i > 0.
func (e *RepeatRunExecutor) RunBlock(block compare.BlockSpec, height int64) (*compare.Result, error) {
	// Build txs exactly once; all apps receive the same bytes.
	// buildTx increments e.sequences as a side-effect, so this loop must
	// run exactly once per RunBlock call to keep nonces correct across blocks.
	var txs [][]byte
	for _, spec := range block.Txs {
		txBytes, err := buildTx(spec, e.txConfig, e.keys, e.accountNums, e.sequences)
		if err != nil {
			return nil, fmt.Errorf("build tx (signer=%s, msg=%s): %w", spec.Signer, spec.Msg, err)
		}
		txs = append(txs, txBytes)
	}

	req := &abci.RequestFinalizeBlock{
		Height: height,
		Txs:    txs,
	}

	// Set lifecycle observers.
	oracleObs := compare.NewBlockObserver(len(txs))
	e.oracle.SetLifecycleObserver(oracleObs)

	probeObservers := make([]*compare.BlockObserver, len(e.probes))
	for i, p := range e.probes {
		obs := compare.NewBlockObserver(len(txs))
		probeObservers[i] = obs
		p.SetLifecycleObserver(obs)
	}

	// Run oracle.
	oracleRes, err := e.oracle.FinalizeBlock(req)
	if err != nil {
		e.unsetObservers()
		return nil, fmt.Errorf("oracle FinalizeBlock: %w", err)
	}

	// Run all probes.
	probeResults := make([]*abci.ResponseFinalizeBlock, len(e.probes))
	for i, p := range e.probes {
		res, err := p.FinalizeBlock(req)
		if err != nil {
			e.unsetObservers()
			return nil, fmt.Errorf("probe[%d] FinalizeBlock: %w", i, err)
		}
		probeResults[i] = res
	}

	e.unsetObservers()

	// Build findings.
	var allFindings []compare.Finding
	txCount := len(txs)

	// F1: oracle vs probe[0] (ProbeIndex = 0).
	f1Findings := repeatCompareResponses(height, oracleRes, probeResults[0], oracleObs, probeObservers[0], txCount, 0)
	allFindings = append(allFindings, f1Findings...)

	// F2: probe[0] vs probe[i] for i = 1..n-1 (ProbeIndex = i).
	for i := 1; i < len(e.probes); i++ {
		f2Findings := repeatCompareResponses(height, probeResults[0], probeResults[i], probeObservers[0], probeObservers[i], txCount, i)
		allFindings = append(allFindings, f2Findings...)
	}

	result := &compare.Result{Height: height}
	if len(allFindings) > 0 {
		result.Verdict = compare.Divergence
		result.Findings = allFindings
	} else {
		result.Verdict = compare.Match
	}

	return result, nil
}

// Close nils all app references and state maps to allow GC.
func (e *RepeatRunExecutor) Close() {
	e.oracle = nil
	e.probes = nil
	e.txConfig = nil
	e.keys = nil
	e.accountNums = nil
	e.sequences = nil
}

// unsetObservers resets lifecycle observers on all apps to the noop observer.
func (e *RepeatRunExecutor) unsetObservers() {
	e.oracle.SetLifecycleObserver(compare.NoopLifecycleObserver{})
	for _, p := range e.probes {
		p.SetLifecycleObserver(compare.NoopLifecycleObserver{})
	}
}

// repeatCompareResponses compares two FinalizeBlock responses (AppHash,
// ErrorCodes, WriteSets) and returns findings. probeIndex is embedded in each
// Finding to distinguish F1 (oracle vs probe[0]) from F2 (probe[0] vs
// probe[i]).
func repeatCompareResponses(
	height int64,
	left, right *abci.ResponseFinalizeBlock,
	leftWS, rightWS compare.WriteSetProvider,
	txCount int,
	probeIndex int,
) []compare.Finding {
	var findings []compare.Finding

	if !bytes.Equal(left.AppHash, right.AppHash) {
		findings = append(findings, compare.NewFinding(
			height, compare.DimAppHash, -1, probeIndex,
			hex.EncodeToString(left.AppHash),
			hex.EncodeToString(right.AppHash),
		))
	}

	n := min(len(left.TxResults), len(right.TxResults))
	if txCount < n {
		n = txCount
	}
	for i := 0; i < n; i++ {
		if left.TxResults[i].Code != right.TxResults[i].Code {
			findings = append(findings, compare.NewFinding(
				height, compare.DimErrorCode, i, probeIndex,
				fmt.Sprintf("%d", left.TxResults[i].Code),
				fmt.Sprintf("%d", right.TxResults[i].Code),
			))
		}
	}

	if leftWS != nil && rightWS != nil {
		for i := 0; i < n; i++ {
			lWS := leftWS.TxWriteSet(i)
			rWS := rightWS.TxWriteSet(i)
			if !compare.EqualStrSlice(lWS, rWS) {
				findings = append(findings, compare.NewFinding(
					height, compare.DimWriteSet, i, probeIndex,
					compare.FormatWriteSet(lWS),
					compare.FormatWriteSet(rWS),
				))
			}
		}
	}

	return findings
}
