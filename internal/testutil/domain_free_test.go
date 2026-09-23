package testutil

import (
	"testing"

	httpdomain "github.com/f0reth/Wirexa/internal/domain/http"
	udpdomain "github.com/f0reth/Wirexa/internal/domain/udp"
)

type freeNode struct {
	Encoding udpdomain.PayloadEncoding
	Tags     map[string][]string
	Children []*freeNode
	Name     string
	Port     int
}

// 基本型・名前付きの基本型・自己参照だけの型は通る。
func TestAssertNoTypesFrom_PassesOnDomainFreeTypes(t *testing.T) {
	rec := &recordingTB{TB: t}
	AssertNoTypesFrom(rec, freeNode{}, DomainPkg)
	if len(rec.errors)+len(rec.fatals) != 0 {
		t.Fatalf("unexpected failures: %v %v", rec.errors, rec.fatals)
	}
}

func TestAssertNoTypesFrom_DetectsDomainStructs(t *testing.T) {
	type inner struct {
		Deep []map[string]*httpdomain.RequestAuth
	}
	tests := map[string]any{
		"field":   struct{ A httpdomain.RequestAuth }{},
		"slice":   struct{ H []httpdomain.KeyValuePair }{},
		"pointer": struct{ P *httpdomain.RequestSettings }{},
		"map value": struct {
			M map[string]httpdomain.KeyValuePair
		}{},
		"nested":    struct{ I []inner }{},
		"top level": httpdomain.SidebarEntry{},
	}
	for name, v := range tests {
		t.Run(name, func(t *testing.T) {
			rec := &recordingTB{TB: t}
			AssertNoTypesFrom(rec, v, DomainPkg)
			if len(rec.errors) == 0 {
				t.Fatal("AssertNoTypesFrom must fail")
			}
		})
	}
}

// 接頭辞はパッケージパスの区切りで照合し、名前が同じ接頭辞で始まるだけの別パッケージは対象にしない。
func TestAssertNoTypesFrom_MatchesWholePathSegments(t *testing.T) {
	rec := &recordingTB{TB: t}
	AssertNoTypesFrom(rec, struct{ A httpdomain.RequestAuth }{}, DomainPkg+"/ht")
	if len(rec.errors) != 0 {
		t.Fatalf("partial segment must not match: %v", rec.errors)
	}
}
