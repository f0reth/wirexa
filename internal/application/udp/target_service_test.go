package udpapp

import (
	"errors"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

// inMemoryTargetRepo は TargetRepository のインメモリモック。
type inMemoryTargetRepo struct {
	targets map[string]*domain.UDPTarget
	loadErr error
	saveErr error
	delErr  error
}

func newTargetRepo(targets ...domain.UDPTarget) *inMemoryTargetRepo {
	r := &inMemoryTargetRepo{targets: make(map[string]*domain.UDPTarget)}
	for i := range targets {
		t := targets[i]
		r.targets[t.ID] = &t
	}
	return r
}

func (r *inMemoryTargetRepo) Load() ([]domain.UDPTarget, error) {
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	result := make([]domain.UDPTarget, 0, len(r.targets))
	for _, t := range r.targets {
		result = append(result, *t)
	}
	return result, nil
}

func (r *inMemoryTargetRepo) Save(target *domain.UDPTarget) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	cp := *target
	r.targets[target.ID] = &cp
	return nil
}

func (r *inMemoryTargetRepo) Delete(id string) error {
	if r.delErr != nil {
		return r.delErr
	}
	delete(r.targets, id)
	return nil
}

func TestTargetService_NewTargetService_LoadsTargets(t *testing.T) {
	repo := newTargetRepo(
		domain.UDPTarget{ID: "t1", Name: "Local", Host: "127.0.0.1", Port: 9000},
		domain.UDPTarget{ID: "t2", Name: "Remote", Host: "192.168.1.1", Port: 8080},
	)
	svc, err := NewTargetService(repo)
	if err != nil {
		t.Fatalf("NewTargetService: %v", err)
	}
	if targets := svc.GetTargets(); len(targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(targets))
	}
}

func TestTargetService_NewTargetService_RepoLoadError(t *testing.T) {
	repo := &inMemoryTargetRepo{
		targets: map[string]*domain.UDPTarget{},
		loadErr: errors.New("read error"),
	}
	_, err := NewTargetService(repo)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTargetService_GetTargets_Empty(t *testing.T) {
	svc, _ := NewTargetService(newTargetRepo())
	if targets := svc.GetTargets(); len(targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(targets))
	}
}

func TestTargetService_GetTargets_ReturnsCopy(t *testing.T) {
	repo := newTargetRepo(domain.UDPTarget{ID: "t1", Name: "Original", Host: "localhost", Port: 9000})
	svc, _ := NewTargetService(repo)

	targets := svc.GetTargets()
	targets[0].Name = "Modified"

	got := svc.GetTargets()
	if got[0].Name != "Original" {
		t.Error("GetTargets should return a copy, not a reference")
	}
}

func TestTargetService_SaveTarget_NewWithoutID(t *testing.T) {
	svc, _ := NewTargetService(newTargetRepo())
	target, err := svc.SaveTarget(domain.UDPTarget{Name: "NoID", Host: "localhost", Port: 9000})
	if err != nil {
		t.Fatalf("SaveTarget: %v", err)
	}
	if target.ID == "" {
		t.Error("expected auto-generated ID, got empty")
	}
	if len(svc.GetTargets()) != 1 {
		t.Error("expected 1 target")
	}
}

func TestTargetService_SaveTarget_NewWithID(t *testing.T) {
	svc, _ := NewTargetService(newTargetRepo())
	target, err := svc.SaveTarget(domain.UDPTarget{ID: "t99", Name: "WithID", Host: "host", Port: 1234})
	if err != nil {
		t.Fatalf("SaveTarget: %v", err)
	}
	if target.ID != "t99" {
		t.Errorf("ID = %q, want t99", target.ID)
	}
}

func TestTargetService_SaveTarget_UpdateExisting(t *testing.T) {
	original := domain.UDPTarget{ID: "t1", Name: "Old", Host: "old-host", Port: 9000}
	svc, _ := NewTargetService(newTargetRepo(original))

	updated, err := svc.SaveTarget(domain.UDPTarget{ID: "t1", Name: "New", Host: "new-host", Port: 8080})
	if err != nil {
		t.Fatalf("SaveTarget: %v", err)
	}
	if updated.Name != "New" {
		t.Errorf("Name = %q, want New", updated.Name)
	}
	targets := svc.GetTargets()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Host != "new-host" {
		t.Errorf("Host = %q, want new-host", targets[0].Host)
	}
}

func TestTargetService_SaveTarget_RepoError(t *testing.T) {
	repo := newTargetRepo()
	repo.saveErr = errors.New("write error")
	svc, _ := NewTargetService(repo)

	_, err := svc.SaveTarget(domain.UDPTarget{Name: "X", Host: "h", Port: 9000})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTargetService_SaveTarget_InvalidReturnsValidationError(t *testing.T) {
	cases := []struct {
		name   string
		target domain.UDPTarget
	}{
		{"empty host", domain.UDPTarget{Name: "X", Host: "", Port: 9000}},
		{"port too low", domain.UDPTarget{Name: "X", Host: "h", Port: 0}},
		{"port too high", domain.UDPTarget{Name: "X", Host: "h", Port: 70000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newTargetRepo()
			svc, _ := NewTargetService(repo)

			_, err := svc.SaveTarget(tc.target)
			var ve *cmn.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError, got %T (%v)", err, err)
			}
			// 不正入力は永続化されない。
			if len(svc.GetTargets()) != 0 {
				t.Errorf("expected 0 targets after invalid save, got %d", len(svc.GetTargets()))
			}
		})
	}
}

func TestTargetService_DeleteTarget_Success(t *testing.T) {
	repo := newTargetRepo(domain.UDPTarget{ID: "t1", Name: "ToDelete", Host: "h", Port: 9000})
	svc, _ := NewTargetService(repo)

	if err := svc.DeleteTarget("t1"); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	if len(svc.GetTargets()) != 0 {
		t.Error("expected 0 targets after delete")
	}
}

func TestTargetService_DeleteTarget_NotFound(t *testing.T) {
	svc, _ := NewTargetService(newTargetRepo())
	err := svc.DeleteTarget("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var nfe *cmn.NotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestTargetService_DeleteTarget_DoesNotCallRepoForMissingID(t *testing.T) {
	// TargetService は in-memory で存在確認するので、repo.Delete は呼ばれない
	repo := newTargetRepo()
	repo.delErr = errors.New("should not be called")
	svc, _ := NewTargetService(repo)

	err := svc.DeleteTarget("nonexistent")
	// repo.Delete は呼ばれないので、NotFoundError を返す（repo エラーではない）
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var nfe *cmn.NotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("expected NotFoundError (not repo error), got %T: %v", err, err)
	}
}
