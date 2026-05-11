package detect

import "strings"

type Category string

const (
	CatTime    Category = "time"
	CatRand    Category = "rand"
	CatIO      Category = "io"
	CatMapIter Category = "map_iter"  // map range where iteration order is observable
	CatPointer Category = "pointer"   // pointer address exposed in output
)

type Finding struct {
	Category Category
	File     string
	Line     int
	FuncName string
	Call     string
	Module   string
}

type ScanResult struct {
	Findings []Finding
	Files    int
}

type Rule struct {
	ImportPath string
	FuncNames  []string
	Category   Category
}

func ModuleFromPath(relPath string) string {
	parts := strings.Split(relPath, "/")
	if len(parts) >= 2 && parts[0] == "x" {
		return parts[1]
	}
	return parts[0]
}
