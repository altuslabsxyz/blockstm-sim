package sdkhook_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// Registries are package-global and registration panics on duplicates, so all
// installer behaviour is exercised in a single test: no-op before registration,
// forwarding after registration, and panic on double registration.
func TestObserverInstallers(t *testing.T) {
	// Before registration, Install* must be silent no-ops.
	require.NotPanics(t, func() {
		sdkhook.InstallConflictObserver(func(compare.ConflictRecord) {})
		sdkhook.InstallConflictObserver(nil)
		sdkhook.InstallExecStatsObserver(func(compare.ExecutionStats) {})
		sdkhook.InstallExecStatsObserver(nil)
	})

	var conflictInstalls []bool // true = install, false = remove
	sdkhook.RegisterConflictObserverInstaller(func(fn func(compare.ConflictRecord)) {
		conflictInstalls = append(conflictInstalls, fn != nil)
	})
	var statsInstalls []bool
	sdkhook.RegisterExecStatsObserverInstaller(func(fn func(compare.ExecutionStats)) {
		statsInstalls = append(statsInstalls, fn != nil)
	})

	sdkhook.InstallConflictObserver(func(compare.ConflictRecord) {})
	sdkhook.InstallConflictObserver(nil)
	require.Equal(t, []bool{true, false}, conflictInstalls)

	sdkhook.InstallExecStatsObserver(func(compare.ExecutionStats) {})
	sdkhook.InstallExecStatsObserver(nil)
	require.Equal(t, []bool{true, false}, statsInstalls)

	// Double registration panics, matching the other sdkhook registries.
	require.Panics(t, func() {
		sdkhook.RegisterConflictObserverInstaller(func(func(compare.ConflictRecord)) {})
	})
	require.Panics(t, func() {
		sdkhook.RegisterExecStatsObserverInstaller(func(func(compare.ExecutionStats)) {})
	})
}
