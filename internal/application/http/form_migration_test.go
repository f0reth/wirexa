package httpapp

import (
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// legacyFormRequest は行フィールド導入前の保存形式（Contents に urlencoded 文字列）を模したリクエストを返す。
func legacyFormRequest(id, content string) *domain.HTTPRequest {
	return &domain.HTTPRequest{
		ID:   id,
		Name: id,
		Body: domain.RequestBody{
			Type:     domain.BodyTypeFormURLEncoded,
			Contents: map[string]string{domain.BodyTypeFormURLEncoded: content},
		},
	}
}

func assertMigrated(t *testing.T, req *domain.HTTPRequest, wantKeys ...string) {
	t.Helper()
	if req == nil {
		t.Fatal("request が nil")
	}
	pairs := req.Body.FormURLEncoded
	if len(pairs) != len(wantKeys) {
		t.Fatalf("行数 = %d, want %d (%+v)", len(pairs), len(wantKeys), pairs)
	}
	for i, want := range wantKeys {
		if pairs[i].Key != want {
			t.Errorf("行[%d].Key = %q, want %q", i, pairs[i].Key, want)
		}
		if !pairs[i].Enabled {
			t.Errorf("行[%d] は有効であるべき", i)
		}
	}
	if _, ok := req.Body.Contents[domain.BodyTypeFormURLEncoded]; ok {
		t.Error("移行後も Contents に form-urlencoded キーが残っている")
	}
}

// TestCollectionService_MigratesLegacyFormBodies は行フィールド導入前に保存された
// リクエストが、読み込み時にフォーム行へ復元されることを確認する。
// 復元し損ねるとフロントには行が無いように見え、autosave で旧データが消える。
func TestCollectionService_MigratesLegacyFormBodies(t *testing.T) {
	repo := &inMemoryRepo{collections: map[string]*domain.Collection{
		"col1": {
			ID:   "col1",
			Name: "Col",
			Items: []*domain.TreeItem{
				{
					Type:    domain.ItemTypeRequest,
					ID:      "top",
					Name:    "top",
					Request: legacyFormRequest("top", "a=1&b=2"),
				},
				{
					Type: domain.ItemTypeFolder,
					ID:   "folder",
					Name: "folder",
					Children: []*domain.TreeItem{
						{
							Type: domain.ItemTypeFolder,
							ID:   "nested",
							Name: "nested",
							Children: []*domain.TreeItem{
								{
									Type:    domain.ItemTypeRequest,
									ID:      "deep",
									Name:    "deep",
									Request: legacyFormRequest("deep", "x=9"),
								},
							},
						},
					},
				},
			},
		},
		domain.RootCollectionID: {
			ID:   domain.RootCollectionID,
			Name: domain.RootCollectionID,
			Items: []*domain.TreeItem{
				{
					Type:    domain.ItemTypeRequest,
					ID:      "rootreq",
					Name:    "rootreq",
					Request: legacyFormRequest("rootreq", "r=1"),
				},
			},
		},
	}}

	svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}

	cols := svc.GetCollections()
	if len(cols) != 1 {
		t.Fatalf("コレクション数 = %d, want 1", len(cols))
	}

	t.Run("トップレベルのリクエスト", func(t *testing.T) {
		assertMigrated(t, cols[0].Items[0].Request, "a", "b")
	})

	t.Run("入れ子フォルダ配下のリクエスト", func(t *testing.T) {
		assertMigrated(t, cols[0].Items[1].Children[0].Children[0].Request, "x")
	})

	// __root__ もキャッシュ経由で返るため移行対象になる。
	t.Run("ルート直下のリクエスト", func(t *testing.T) {
		items := svc.GetRootItems()
		if len(items) != 1 {
			t.Fatalf("ルートアイテム数 = %d, want 1", len(items))
		}
		assertMigrated(t, items[0].Request, "r")
	})
}
