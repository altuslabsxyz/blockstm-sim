package detect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRulesLookup(t *testing.T) {
	idx := DefaultRules().index()

	cat, ok := idx.Lookup("time", "Now")
	require.True(t, ok)
	require.Equal(t, CatTime, cat)

	cat, ok = idx.Lookup("time", "Since")
	require.True(t, ok)
	require.Equal(t, CatTime, cat)

	cat, ok = idx.Lookup("crypto/rand", "Read")
	require.True(t, ok)
	require.Equal(t, CatRand, cat)

	cat, ok = idx.Lookup("math/rand", "Intn")
	require.True(t, ok)
	require.Equal(t, CatRand, cat)

	cat, ok = idx.Lookup("os", "Getenv")
	require.True(t, ok)
	require.Equal(t, CatIO, cat)

	cat, ok = idx.Lookup("net/http", "Get")
	require.True(t, ok)
	require.Equal(t, CatIO, cat)

	_, ok = idx.Lookup("time", "Duration")
	require.False(t, ok)

	_, ok = idx.Lookup("fmt", "Println")
	require.False(t, ok)
}

func TestDefaultRulesCompleteness(t *testing.T) {
	rules := DefaultRules()
	var timeCount, randCount, ioCount int
	for _, r := range rules {
		switch r.Category {
		case CatTime:
			timeCount += len(r.FuncNames)
		case CatRand:
			randCount += len(r.FuncNames)
		case CatIO:
			ioCount += len(r.FuncNames)
		}
	}
	require.Greater(t, timeCount, 0, "should have time rules")
	require.Greater(t, randCount, 0, "should have rand rules")
	require.Greater(t, ioCount, 0, "should have io rules")
}
