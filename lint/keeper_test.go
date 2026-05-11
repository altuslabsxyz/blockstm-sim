package lint_test

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/lint"
)

func scan(t *testing.T, src string) []lint.Finding {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "keeper.go", src, 0)
	require.NoError(t, err)
	return lint.NewScanner().ScanFile(fset, f, "x/bank/keeper.go")
}

// ---------------------------------------------------------------------------
// keeper_field findings
// ---------------------------------------------------------------------------

func TestScan_KeeperFieldAssignment(t *testing.T) {
	src := `package keeper
type Keeper struct{ cache map[string]int64 }
func (k *Keeper) Set(ctx sdk.Context, key string, val int64) {
	k.cache[key] = val
}`
	findings := scan(t, src)
	require.Len(t, findings, 1)
	require.Equal(t, lint.KindKeeperField, findings[0].Kind)
	require.Equal(t, "cache", findings[0].Target)
	require.Equal(t, "Set", findings[0].Method)
}

func TestScan_KeeperFieldIncDec(t *testing.T) {
	src := `package keeper
type Keeper struct{ counter int64 }
func (k *Keeper) Increment(ctx sdk.Context) {
	k.counter++
}`
	findings := scan(t, src)
	require.Len(t, findings, 1)
	require.Equal(t, lint.KindKeeperField, findings[0].Kind)
	require.Equal(t, "counter", findings[0].Target)
}

func TestScan_KeeperFieldDirect(t *testing.T) {
	src := `package keeper
type Keeper struct{ sharedValue int64 }
func (k *Keeper) Store(ctx sdk.Context, v int64) {
	k.sharedValue = v
}`
	findings := scan(t, src)
	require.Len(t, findings, 1)
	require.Equal(t, lint.KindKeeperField, findings[0].Kind)
	require.Equal(t, "sharedValue", findings[0].Target)
}

// ---------------------------------------------------------------------------
// Safe fields — must NOT produce findings
// ---------------------------------------------------------------------------

func TestScan_SafeFields_NoFinding(t *testing.T) {
	src := `package keeper
type Keeper struct{
	storeKey   storetypes.StoreKey
	cdc        codec.BinaryCodec
	authority  string
	bankKeeper types.BankKeeper
}
func (k *Keeper) Process(ctx sdk.Context) {
	store := ctx.KVStore(k.storeKey)
	_ = store
	_ = k.cdc
	_ = k.authority
	_ = k.bankKeeper
}`
	findings := scan(t, src)
	require.Empty(t, findings, "reads of safe fields must not be flagged")
}

// ---------------------------------------------------------------------------
// pkg_var findings
// ---------------------------------------------------------------------------

func TestScan_PkgVarAssignment(t *testing.T) {
	src := `package keeper
var globalCounter int64
type Keeper struct{}
func (k *Keeper) Handle(ctx sdk.Context) {
	globalCounter++
}`
	findings := scan(t, src)
	require.Len(t, findings, 1)
	require.Equal(t, lint.KindPkgVar, findings[0].Kind)
	require.Equal(t, "globalCounter", findings[0].Target)
}

func TestScan_PkgVar_DirectAssign(t *testing.T) {
	src := `package keeper
var lastSeen string
type Keeper struct{}
func (k *Keeper) Track(ctx sdk.Context, addr string) {
	lastSeen = addr
}`
	findings := scan(t, src)
	require.Len(t, findings, 1)
	require.Equal(t, lint.KindPkgVar, findings[0].Kind)
	require.Equal(t, "lastSeen", findings[0].Target)
}

// ---------------------------------------------------------------------------
// Non-context methods — still flagged (helper callers can be ctx-bearing)
// ---------------------------------------------------------------------------

func TestScan_NonContextMethod_StillFlagged(t *testing.T) {
	src := `package keeper
type Keeper struct{ cache map[string]int64 }
// No ctx param but called from ctx-bearing methods — still flagged.
func (k *Keeper) warmCache(key string, val int64) {
	k.cache[key] = val
}`
	findings := scan(t, src)
	require.Len(t, findings, 1)
	require.Equal(t, lint.KindKeeperField, findings[0].Kind)
}

// ---------------------------------------------------------------------------
// Constructor methods — must NOT be flagged
// ---------------------------------------------------------------------------

func TestScan_Constructor_NoFinding(t *testing.T) {
	src := `package keeper
type Keeper struct{ cache map[string]int64 }
func NewKeeper() *Keeper {
	k := &Keeper{}
	k.cache = make(map[string]int64)
	return k
}`
	findings := scan(t, src)
	require.Empty(t, findings, "constructors must not be flagged")
}

// ---------------------------------------------------------------------------
// Plain functions (no receiver) — must NOT flag keeper_field
// ---------------------------------------------------------------------------

func TestScan_PlainFunction_NoKeeperField(t *testing.T) {
	src := `package keeper
var counter int64
func process(ctx sdk.Context) {
	counter++
}`
	findings := scan(t, src)
	// pkg_var finding expected, no keeper_field
	for _, f := range findings {
		require.NotEqual(t, lint.KindKeeperField, f.Kind)
	}
}

// ---------------------------------------------------------------------------
// Module inference
// ---------------------------------------------------------------------------

func TestScan_ModuleFromPath(t *testing.T) {
	cases := []struct {
		path   string
		module string
	}{
		{"x/bank/keeper/keeper.go", "bank"},
		{"x/staking/keeper.go", "staking"},
		{"app/app.go", "app"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.module, lint.ModuleFromPath(tc.path))
	}
}
