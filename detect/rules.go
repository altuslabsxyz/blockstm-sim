package detect

type RuleSet []Rule

func DefaultRules() RuleSet {
	return RuleSet{
		{
			ImportPath: "time",
			FuncNames:  []string{"Now", "Since", "Until", "NewTicker", "NewTimer", "After", "AfterFunc", "Tick"},
			Category:   CatTime,
		},
		{
			ImportPath: "crypto/rand",
			FuncNames:  []string{"Read", "Int", "Prime"},
			Category:   CatRand,
		},
		{
			ImportPath: "math/rand",
			FuncNames: []string{
				"Int", "Intn", "Int31", "Int31n", "Int63", "Int63n",
				"Float32", "Float64", "Uint32", "Uint64",
				"NormFloat64", "ExpFloat64", "Perm", "Shuffle", "Read",
			},
			Category: CatRand,
		},
		{
			ImportPath: "os",
			FuncNames: []string{
				"Open", "OpenFile", "ReadFile", "WriteFile", "Create",
				"Getenv", "LookupEnv", "Environ",
				"ReadDir", "Mkdir", "MkdirAll", "Remove", "RemoveAll",
				"Stat", "Lstat",
			},
			Category: CatIO,
		},
		{
			ImportPath: "net",
			FuncNames:  []string{"Dial", "Listen", "DialTimeout", "DialTCP", "DialUDP", "DialIP", "ListenTCP", "ListenUDP"},
			Category:   CatIO,
		},
		{
			ImportPath: "net/http",
			FuncNames:  []string{"Get", "Post", "Head", "NewRequest"},
			Category:   CatIO,
		},
	}
}

type ruleIndex struct {
	m map[string]map[string]Category
}

func (rs RuleSet) index() *ruleIndex {
	idx := &ruleIndex{m: make(map[string]map[string]Category)}
	for _, r := range rs {
		if idx.m[r.ImportPath] == nil {
			idx.m[r.ImportPath] = make(map[string]Category)
		}
		for _, fn := range r.FuncNames {
			idx.m[r.ImportPath][fn] = r.Category
		}
	}
	return idx
}

func (idx *ruleIndex) Lookup(importPath, funcName string) (Category, bool) {
	funcs, ok := idx.m[importPath]
	if !ok {
		return "", false
	}
	cat, ok := funcs[funcName]
	return cat, ok
}
