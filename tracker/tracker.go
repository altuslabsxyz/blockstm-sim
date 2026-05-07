// Package tracker provides reflection-based OutOfKVStore mutation detection
// for any cosmos-sdk keeper or module struct without requiring modification of
// the target type.
package tracker

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"unsafe"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

var _ compare.MutationTracker = (*KeeperReflectTracker)(nil)

// KeeperReflectTracker implements compare.MutationTracker via reflection.
// It snapshots the mutable, non-KVStore fields of any keeper or module instance,
// detecting out-of-KVStore state changes without any marker interface on the target.
type KeeperReflectTracker struct {
	name string
	obj  any
}

// New wraps any keeper or module instance. The tracker name is derived from
// the concrete type's package path and name.
func New(obj any) *KeeperReflectTracker {
	t := reflect.TypeOf(obj)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	name := "unknown"
	if t != nil {
		name = t.PkgPath() + "." + t.Name()
	}
	return &KeeperReflectTracker{name: name, obj: obj}
}

func (t *KeeperReflectTracker) TrackerName() string { return t.name }

func (t *KeeperReflectTracker) SnapshotOutOfKVStoreState() []byte {
	v := reflect.ValueOf(t.obj)
	visited := make(map[uintptr]bool)
	var buf bytes.Buffer
	snapshotVal(v, &buf, visited, 0)
	if buf.Len() == 0 {
		return nil
	}
	return buf.Bytes()
}

// maxDepth limits recursion to guard against unexpectedly deep object graphs.
const maxDepth = 8

func snapshotVal(v reflect.Value, buf *bytes.Buffer, visited map[uintptr]bool, depth int) {
	if depth > maxDepth {
		return
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		ptr := v.Pointer()
		if visited[ptr] {
			return
		}
		visited[ptr] = true
		snapshotVal(v.Elem(), buf, visited, depth+1)

	case reflect.Struct:
		snapshotStruct(v, buf, visited, depth)

	case reflect.Map:
		if v.IsNil() {
			return
		}
		snapshotMap(v, buf, visited, depth)

	case reflect.Slice:
		if v.IsNil() {
			return
		}
		snapshotSlice(v, buf, visited, depth)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		writeUint64(buf, uint64(v.Int()))

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		writeUint64(buf, v.Uint())

	case reflect.Bool:
		if v.Bool() {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}

	case reflect.String:
		s := v.String()
		writeUint64(buf, uint64(len(s)))
		buf.WriteString(s)
	}
}

var (
	mutexType   = reflect.TypeOf(sync.Mutex{})
	rwMutexType = reflect.TypeOf(sync.RWMutex{})
)

func snapshotStruct(v reflect.Value, buf *bytes.Buffer, visited map[uintptr]bool, depth int) {
	t := v.Type()
	if t == mutexType || t == rwMutexType {
		return
	}

	// Interface-boxed or otherwise non-addressable structs must be copied to
	// heap before their unexported fields can be accessed via unsafe.
	if !v.CanAddr() {
		ptr := reflect.New(t)
		ptr.Elem().Set(v)
		v = ptr.Elem()
	}

	for i := 0; i < v.NumField(); i++ {
		f := t.Field(i)
		fv := v.Field(i)

		if shouldSkipField(f) {
			continue
		}

		accessible := makeAccessible(fv)
		if !accessible.IsValid() {
			continue
		}

		fmt.Fprintf(buf, "[%s]", f.Name)
		snapshotVal(accessible, buf, visited, depth+1)
	}
}

// makeAccessible returns an accessible view of v, using unsafe for unexported fields.
func makeAccessible(v reflect.Value) reflect.Value {
	if v.CanInterface() {
		return v
	}
	if !v.CanAddr() {
		return reflect.Value{}
	}
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}

func snapshotMap(v reflect.Value, buf *bytes.Buffer, visited map[uintptr]bool, depth int) {
	keys := v.MapKeys()
	strs := make([]string, len(keys))
	keyByStr := make(map[string]reflect.Value, len(keys))
	for i, k := range keys {
		s := fmt.Sprintf("%v", k)
		strs[i] = s
		keyByStr[s] = k
	}
	sort.Strings(strs)

	for _, s := range strs {
		val := v.MapIndex(keyByStr[s])
		fmt.Fprintf(buf, "{%s:", s)
		snapshotVal(val, buf, visited, depth+1)
		buf.WriteByte('}')
	}
}

func snapshotSlice(v reflect.Value, buf *bytes.Buffer, visited map[uintptr]bool, depth int) {
	writeUint64(buf, uint64(v.Len()))
	for i := 0; i < v.Len(); i++ {
		snapshotVal(v.Index(i), buf, visited, depth+1)
	}
}

func writeUint64(buf *bytes.Buffer, v uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	buf.Write(b[:])
}
