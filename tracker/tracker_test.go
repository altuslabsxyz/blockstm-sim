package tracker

import (
	"bytes"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// ── shouldSkipField ─────────────────────────────────────────────────────────

func TestShouldSkipField_Func(t *testing.T) {
	type S struct{ F func() }
	require.True(t, shouldSkipField(reflect.TypeOf(S{}).Field(0)))
}

func TestShouldSkipField_Chan(t *testing.T) {
	type S struct{ C chan int }
	require.True(t, shouldSkipField(reflect.TypeOf(S{}).Field(0)))
}

func TestShouldSkipField_Interface(t *testing.T) {
	type S struct{ I interface{} }
	require.True(t, shouldSkipField(reflect.TypeOf(S{}).Field(0)))
}

func TestShouldSkipField_IntNotSkipped(t *testing.T) {
	type S struct{ N int }
	require.False(t, shouldSkipField(reflect.TypeOf(S{}).Field(0)))
}

func TestShouldSkipField_PtrToIntNotSkipped(t *testing.T) {
	type S struct{ P *int }
	require.False(t, shouldSkipField(reflect.TypeOf(S{}).Field(0)))
}

// ── New / TrackerName ────────────────────────────────────────────────────────

func TestNew_NameContainsTypeName(t *testing.T) {
	type MyKeeper struct{}
	tr := New(&MyKeeper{})
	require.Contains(t, tr.TrackerName(), "MyKeeper")
}

func TestNew_NameContainsPkgPath(t *testing.T) {
	type SomeKeeper struct{}
	tr := New(&SomeKeeper{})
	// Package path for this test file is the tracker package itself.
	require.True(t, strings.Contains(tr.TrackerName(), "tracker"), tr.TrackerName())
}

func TestNew_NilPointer_NameNotUnknown(t *testing.T) {
	type K struct{ N int }
	// Typed nil: type info is still available.
	tr := New((*K)(nil))
	require.Contains(t, tr.TrackerName(), "K")
}

// ── SnapshotOutOfKVStoreState: basic types ───────────────────────────────────

func TestSnapshot_EmptyStruct_IsNil(t *testing.T) {
	type Empty struct{}
	snap := New(&Empty{}).SnapshotOutOfKVStoreState()
	require.Nil(t, snap)
}

func TestSnapshot_NilPointer_IsNil(t *testing.T) {
	type K struct{ N int }
	snap := New((*K)(nil)).SnapshotOutOfKVStoreState()
	require.Nil(t, snap)
}

func TestSnapshot_IntField_NonNil(t *testing.T) {
	type S struct{ N int }
	s := &S{N: 42}
	snap := New(s).SnapshotOutOfKVStoreState()
	require.NotNil(t, snap)
}

func TestSnapshot_IntField_DetectsChange(t *testing.T) {
	type S struct{ N int }
	s := &S{N: 1}
	tr := New(s)
	before := tr.SnapshotOutOfKVStoreState()
	s.N = 2
	after := tr.SnapshotOutOfKVStoreState()
	require.False(t, bytes.Equal(before, after))
}

func TestSnapshot_BoolField(t *testing.T) {
	type S struct{ B bool }
	s := &S{B: false}
	tr := New(s)
	off := tr.SnapshotOutOfKVStoreState()
	s.B = true
	on := tr.SnapshotOutOfKVStoreState()
	require.False(t, bytes.Equal(off, on))
}

func TestSnapshot_StringField(t *testing.T) {
	type S struct{ Label string }
	s := &S{Label: "hello"}
	tr := New(s)
	snap1 := tr.SnapshotOutOfKVStoreState()
	s.Label = "world"
	snap2 := tr.SnapshotOutOfKVStoreState()
	require.False(t, bytes.Equal(snap1, snap2))
}

func TestSnapshot_SliceField(t *testing.T) {
	type S struct{ Items []int }
	s := &S{Items: []int{1, 2, 3}}
	tr := New(s)
	snap1 := tr.SnapshotOutOfKVStoreState()
	s.Items = append(s.Items, 4)
	snap2 := tr.SnapshotOutOfKVStoreState()
	require.False(t, bytes.Equal(snap1, snap2))
}

// ── Determinism ──────────────────────────────────────────────────────────────

func TestSnapshot_Deterministic_Repeated(t *testing.T) {
	type S struct{ N int }
	s := &S{N: 7}
	tr := New(s)
	for i := 0; i < 5; i++ {
		require.Equal(t, tr.SnapshotOutOfKVStoreState(), tr.SnapshotOutOfKVStoreState())
	}
}

func TestSnapshot_MapField_Deterministic(t *testing.T) {
	type S struct{ M map[string]int }
	s := &S{M: map[string]int{"z": 3, "a": 1, "m": 2}}
	tr := New(s)
	snap1 := tr.SnapshotOutOfKVStoreState()
	snap2 := tr.SnapshotOutOfKVStoreState()
	require.Equal(t, snap1, snap2, "map snapshot must be order-independent")
}

func TestSnapshot_MapField_DetectsChange(t *testing.T) {
	type S struct{ M map[string]int }
	s := &S{M: map[string]int{"a": 1}}
	tr := New(s)
	before := tr.SnapshotOutOfKVStoreState()
	s.M["b"] = 2
	after := tr.SnapshotOutOfKVStoreState()
	require.False(t, bytes.Equal(before, after))
}

// ── Safety: cycles and depth ─────────────────────────────────────────────────

func TestSnapshot_CycleDetection_NoPanic(t *testing.T) {
	type Node struct {
		Val  int
		Next *Node
	}
	n := &Node{Val: 1}
	n.Next = n
	require.NotPanics(t, func() {
		New(n).SnapshotOutOfKVStoreState()
	})
}

func TestSnapshot_DeepNesting_NoPanic(t *testing.T) {
	type Box struct{ Inner *Box }
	root := &Box{}
	cur := root
	for i := 0; i < 20; i++ {
		cur.Inner = &Box{}
		cur = cur.Inner
	}
	require.NotPanics(t, func() {
		New(root).SnapshotOutOfKVStoreState()
	})
}

// ── Filtering: mutex and function fields ─────────────────────────────────────

func TestSnapshot_OnceNotTracked(t *testing.T) {
	type S struct {
		once sync.Once
		N    int
	}
	s := &S{N: 5}
	tr := New(s)
	snap1 := tr.SnapshotOutOfKVStoreState()
	s.once.Do(func() {}) // transitions done: 0 → 1
	snap2 := tr.SnapshotOutOfKVStoreState()
	require.Equal(t, snap1, snap2, "sync.Once state must not affect snapshot")
}

func TestSnapshot_MutexNotTracked(t *testing.T) {
	type S struct {
		mu sync.Mutex
		N  int
	}
	s := &S{N: 5}
	tr := New(s)
	snap1 := tr.SnapshotOutOfKVStoreState()
	s.mu.Lock()
	snap2 := tr.SnapshotOutOfKVStoreState()
	s.mu.Unlock()
	require.Equal(t, snap1, snap2, "mutex state must not affect snapshot")
}

func TestSnapshot_FuncFieldNotTracked(t *testing.T) {
	type S struct {
		Fn func()
		N  int
	}
	called := false
	s := &S{Fn: func() { called = true }, N: 3}
	tr := New(s)
	snap := tr.SnapshotOutOfKVStoreState()
	// Snapshot should reflect N but not panic trying to snapshot Fn.
	require.NotNil(t, snap)
	require.False(t, called)
}

// ── Unexported fields via unsafe ─────────────────────────────────────────────

func TestSnapshot_UnexportedInt_Tracked(t *testing.T) {
	type S struct{ n int }
	s := &S{n: 7}
	tr := New(s)
	snap1 := tr.SnapshotOutOfKVStoreState()
	require.NotNil(t, snap1)
	s.n = 99
	snap2 := tr.SnapshotOutOfKVStoreState()
	require.False(t, bytes.Equal(snap1, snap2), "unexported field change must be detected")
}
