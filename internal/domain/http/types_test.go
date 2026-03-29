package httpdomain

import "testing"

// buildTree constructs a test collection:
//
//	col
//	├── f1 (folder)
//	│   ├── r1 (request)
//	│   └── f2 (folder)
//	│       └── r2 (request)
//	└── r3 (request)
func buildTree() *Collection {
	r2 := &TreeItem{Type: ItemTypeRequest, ID: "r2", Name: "Request2", Children: []*TreeItem{}}
	f2 := &TreeItem{Type: ItemTypeFolder, ID: "f2", Name: "Folder2", Children: []*TreeItem{r2}}
	r1 := &TreeItem{Type: ItemTypeRequest, ID: "r1", Name: "Request1", Children: []*TreeItem{}}
	f1 := &TreeItem{Type: ItemTypeFolder, ID: "f1", Name: "Folder1", Children: []*TreeItem{r1, f2}}
	r3 := &TreeItem{Type: ItemTypeRequest, ID: "r3", Name: "Request3", Children: []*TreeItem{}}
	return &Collection{ID: "col1", Items: []*TreeItem{f1, r3}}
}

func TestCollection_FindNode_Found(t *testing.T) {
	tests := []struct {
		id           string
		wantParentID string // "" means parent should be nil (root)
		hasParent    bool
	}{
		{"f1", "", false},  // root folder
		{"r3", "", false},  // root request
		{"r1", "f1", true}, // nested under f1
		{"f2", "f1", true}, // nested folder under f1
		{"r2", "f2", true}, // deeply nested
	}

	for _, tt := range tests {
		t.Run("find_"+tt.id, func(t *testing.T) {
			col := buildTree()
			node, parent, ok := col.FindNode(tt.id)
			if !ok {
				t.Fatalf("FindNode(%q) returned false", tt.id)
			}
			if node.ID != tt.id {
				t.Errorf("node.ID = %q, want %q", node.ID, tt.id)
			}
			if tt.hasParent {
				if parent == nil {
					t.Fatalf("expected parent %q, got nil", tt.wantParentID)
				}
				if parent.ID != tt.wantParentID {
					t.Errorf("parent.ID = %q, want %q", parent.ID, tt.wantParentID)
				}
			} else if parent != nil {
				t.Errorf("expected nil parent, got parent.ID=%q", parent.ID)
			}
		})
	}
}

func TestCollection_FindNode_NotFound(t *testing.T) {
	col := buildTree()
	_, _, ok := col.FindNode("nonexistent")
	if ok {
		t.Error("expected FindNode to return false for unknown ID")
	}
}

func TestCollection_FindNode_EmptyCollection(t *testing.T) {
	col := &Collection{ID: "empty", Items: []*TreeItem{}}
	_, _, ok := col.FindNode("any")
	if ok {
		t.Error("expected false for empty collection")
	}
}

func TestCollection_RemoveNode_RootItem(t *testing.T) {
	col := buildTree()
	// Remove r3 (root request)
	ok := col.RemoveNode("r3")
	if !ok {
		t.Fatal("expected RemoveNode to return true")
	}
	if _, _, found := col.FindNode("r3"); found {
		t.Error("r3 should not exist after removal")
	}
	// f1 should still exist
	if _, _, found := col.FindNode("f1"); !found {
		t.Error("f1 should still exist")
	}
	if len(col.Items) != 1 {
		t.Errorf("expected 1 root item, got %d", len(col.Items))
	}
}

func TestCollection_RemoveNode_RootFolder(t *testing.T) {
	col := buildTree()
	// Removing f1 should remove entire subtree
	ok := col.RemoveNode("f1")
	if !ok {
		t.Fatal("expected RemoveNode to return true")
	}
	if len(col.Items) != 1 {
		t.Errorf("expected 1 root item, got %d", len(col.Items))
	}
	if col.Items[0].ID != "r3" {
		t.Errorf("expected r3, got %q", col.Items[0].ID)
	}
	// r1, f2, r2 should all be gone
	for _, id := range []string{"f1", "r1", "f2", "r2"} {
		if _, _, found := col.FindNode(id); found {
			t.Errorf("%q should not exist after removing parent f1", id)
		}
	}
}

func TestCollection_RemoveNode_NestedItem(t *testing.T) {
	col := buildTree()
	ok := col.RemoveNode("r1")
	if !ok {
		t.Fatal("expected RemoveNode to return true")
	}
	if _, _, found := col.FindNode("r1"); found {
		t.Error("r1 should not exist after removal")
	}
	// Parent f1 should still exist with only f2
	node, _, found := col.FindNode("f1")
	if !found {
		t.Fatal("f1 should still exist")
	}
	if len(node.Children) != 1 {
		t.Errorf("f1 should have 1 child, got %d", len(node.Children))
	}
}

func TestCollection_RemoveNode_DeeplyNested(t *testing.T) {
	col := buildTree()
	ok := col.RemoveNode("r2")
	if !ok {
		t.Fatal("expected RemoveNode to return true")
	}
	if _, _, found := col.FindNode("r2"); found {
		t.Error("r2 should not exist after removal")
	}
	// f2 should be empty now
	f2, _, found := col.FindNode("f2")
	if !found {
		t.Fatal("f2 should still exist")
	}
	if len(f2.Children) != 0 {
		t.Errorf("f2 should have 0 children, got %d", len(f2.Children))
	}
}

func TestCollection_RemoveNode_NotFound(t *testing.T) {
	col := buildTree()
	ok := col.RemoveNode("nonexistent")
	if ok {
		t.Error("expected RemoveNode to return false for unknown ID")
	}
}

func TestCollection_RemoveNode_EmptyCollection(t *testing.T) {
	col := &Collection{ID: "empty", Items: []*TreeItem{}}
	ok := col.RemoveNode("any")
	if ok {
		t.Error("expected false for empty collection")
	}
}
