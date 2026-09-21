package httpapp

import (
	"errors"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// recordingLogger は Error の呼び出し回数だけを数えるテスト用ロガー。
type recordingLogger struct{ errors int }

func (l *recordingLogger) Info(_ string, _ ...any)  {}
func (l *recordingLogger) Debug(_ string, _ ...any) {}
func (l *recordingLogger) Error(_ string, _ ...any) { l.errors++ }

func col(id, name string) *domain.Collection {
	return &domain.Collection{ID: id, Name: name, Items: []*domain.TreeItem{}}
}

func TestUnitOfWork_AllSucceed(t *testing.T) {
	repo := newFakeRepo(col("c1", "Old1"), col("c2", "Old2"))
	uow := &unitOfWork{repo: repo}

	uow.SaveCollection(repo.snapshot("c1"), col("c1", "New1"))
	uow.SaveCollection(repo.snapshot("c2"), col("c2", "New2"))

	if err := uow.Err(); err != nil {
		t.Fatalf("Err = %v, want nil", err)
	}
	if got := repo.snapshot("c1").Name; got != "New1" {
		t.Errorf("c1 name = %q, want New1", got)
	}
	if got := repo.snapshot("c2").Name; got != "New2" {
		t.Errorf("c2 name = %q, want New2", got)
	}
	if got := repo.saveOrder(); len(got) != 2 || got[0] != "c1" || got[1] != "c2" {
		t.Errorf("save order = %v, want [c1 c2]", got)
	}
}

func TestUnitOfWork_SecondFails_RollbackRestoresFirst(t *testing.T) {
	repo := newFakeRepo(col("c1", "Old1"), col("c2", "Old2"))
	prev1, prev2 := repo.snapshot("c1"), repo.snapshot("c2")
	repo.failSave["c2"] = errors.New("save error")
	uow := &unitOfWork{repo: repo}

	uow.SaveCollection(prev1, col("c1", "New1"))
	uow.SaveCollection(prev2, col("c2", "New2"))
	// 失敗後の操作はスキップされる。
	uow.SaveCollection(prev1, col("c3", "New3"))

	if uow.Err() == nil {
		t.Fatal("Err = nil, want the save error")
	}
	if repo.snapshot("c3") != nil {
		t.Error("c3 was written after the unit of work had already failed")
	}

	uow.Rollback()
	if got := repo.snapshot("c1").Name; got != "Old1" {
		t.Errorf("c1 name after rollback = %q, want Old1", got)
	}
	if got := repo.snapshot("c2").Name; got != "Old2" {
		t.Errorf("c2 name after rollback = %q, want Old2", got)
	}
}

func TestUnitOfWork_RollbackOfNewCollectionDeletesIt(t *testing.T) {
	repo := newFakeRepo()
	repo.failSave["c2"] = errors.New("save error")
	uow := &unitOfWork{repo: repo}

	uow.SaveCollection(nil, col("c1", "New1"))
	uow.SaveCollection(nil, col("c2", "New2"))
	if uow.Err() == nil {
		t.Fatal("Err = nil, want the save error")
	}

	uow.Rollback()
	if repo.snapshot("c1") != nil {
		t.Error("c1 should have been deleted by the rollback of a new collection")
	}
}

func TestUnitOfWork_DeleteCollection_RollbackRestoresIt(t *testing.T) {
	repo := newFakeRepo(col("c1", "Old1"), col("c2", "Old2"))
	repo.failSave["c2"] = errors.New("save error")
	uow := &unitOfWork{repo: repo}

	uow.DeleteCollection(repo.snapshot("c1"))
	uow.SaveCollection(repo.snapshot("c2"), col("c2", "New2"))
	if uow.Err() == nil {
		t.Fatal("Err = nil, want the save error")
	}

	uow.Rollback()
	if got := repo.snapshot("c1"); got == nil || got.Name != "Old1" {
		t.Errorf("c1 after rollback = %v, want the deleted collection restored", got)
	}
}

func TestUnitOfWork_RollbackFailure_IsLoggedAndContinues(t *testing.T) {
	repo := newFakeRepo(col("c1", "Old1"), col("c2", "Old2"), col("c3", "Old3"))
	prev1, prev2, prev3 := repo.snapshot("c1"), repo.snapshot("c2"), repo.snapshot("c3")
	logger := &recordingLogger{}
	uow := &unitOfWork{repo: repo, logger: logger}

	uow.SaveCollection(prev1, col("c1", "New1"))
	uow.SaveCollection(prev2, col("c2", "New2"))
	// c2 の巻き戻しだけが失敗し、c1 の巻き戻しは実行される状態を作る。
	repo.failSave["c2"] = errors.New("save error")
	uow.SaveCollection(prev3, col("c3", "New3"))
	if uow.Err() != nil {
		t.Fatalf("Err = %v, want nil before the rollback", uow.Err())
	}
	repo.failSave["c3"] = errors.New("save error")

	uow.Rollback()
	if logger.errors != 2 {
		t.Errorf("logged errors = %d, want 2 (c3 と c2 の巻き戻し失敗)", logger.errors)
	}
	if got := repo.snapshot("c1").Name; got != "Old1" {
		t.Errorf("c1 name after rollback = %q, want Old1 (巻き戻しは残りを続行する)", got)
	}
}

func TestUnitOfWork_RollbackWithoutLogger(t *testing.T) {
	// logger が nil でも巻き戻し失敗で panic しないこと。
	repo := newFakeRepo(col("c1", "Old1"))
	prev := repo.snapshot("c1")
	uow := &unitOfWork{repo: repo}
	uow.SaveCollection(prev, col("c1", "New1"))
	repo.failSave["c1"] = errors.New("save error")
	uow.Rollback()
}
