package mqttapp

import (
	"errors"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// inMemoryProfileRepo は ProfileRepository のインメモリモック。
type inMemoryProfileRepo struct {
	profiles map[string]*domain.BrokerProfile
	loadErr  error
	saveErr  error
	delErr   error
}

func newProfileRepo(profiles ...domain.BrokerProfile) *inMemoryProfileRepo {
	r := &inMemoryProfileRepo{profiles: make(map[string]*domain.BrokerProfile)}
	for i := range profiles {
		p := profiles[i]
		r.profiles[p.ID] = &p
	}
	return r
}

func (r *inMemoryProfileRepo) Load() ([]domain.BrokerProfile, error) {
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	result := make([]domain.BrokerProfile, 0, len(r.profiles))
	for _, p := range r.profiles {
		result = append(result, *p)
	}
	return result, nil
}

func (r *inMemoryProfileRepo) Save(profile *domain.BrokerProfile) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	cp := *profile
	r.profiles[profile.ID] = &cp
	return nil
}

func (r *inMemoryProfileRepo) Delete(id string) error {
	if r.delErr != nil {
		return r.delErr
	}
	delete(r.profiles, id)
	return nil
}

func TestProfileService_NewProfileService_LoadsProfiles(t *testing.T) {
	repo := newProfileRepo(
		domain.BrokerProfile{ID: "p1", Name: "Local"},
		domain.BrokerProfile{ID: "p2", Name: "Remote"},
	)
	svc, err := NewProfileService(repo)
	if err != nil {
		t.Fatalf("NewProfileService: %v", err)
	}
	profiles := svc.GetProfiles()
	if len(profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestProfileService_NewProfileService_RepoLoadError(t *testing.T) {
	repo := &inMemoryProfileRepo{
		profiles: map[string]*domain.BrokerProfile{},
		loadErr:  errors.New("disk error"),
	}
	_, err := NewProfileService(repo)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProfileService_NewProfileService_NilLoadReturnsEmpty(t *testing.T) {
	// repo.Load() が nil を返した場合、空スライスとして扱う
	repo := &inMemoryProfileRepo{profiles: map[string]*domain.BrokerProfile{}}
	svc, err := NewProfileService(repo)
	if err != nil {
		t.Fatalf("NewProfileService: %v", err)
	}
	if profiles := svc.GetProfiles(); len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestProfileService_GetProfiles_ReturnsCopy(t *testing.T) {
	repo := newProfileRepo(domain.BrokerProfile{ID: "p1", Name: "Original"})
	svc, _ := NewProfileService(repo)

	profiles := svc.GetProfiles()
	profiles[0].Name = "Modified"

	// 元のサービス内データが変更されていないことを確認
	got := svc.GetProfiles()
	if got[0].Name != "Original" {
		t.Error("GetProfiles should return a copy, not a reference")
	}
}

func TestProfileService_SaveProfile_AddNew(t *testing.T) {
	svc, _ := NewProfileService(newProfileRepo())
	// 新規作成は ID を空で渡す (ID はサーバ側で採番する)。
	p := domain.BrokerProfile{Name: "NewProfile", Broker: "tcp://localhost:1883"}
	saved, err := svc.SaveProfile(p)
	if err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	if saved.ID == "" {
		t.Error("expected server-generated ID, got empty")
	}
	if saved.Name != "NewProfile" {
		t.Errorf("Name = %q, want %q", saved.Name, "NewProfile")
	}
	profiles := svc.GetProfiles()
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].ID != saved.ID {
		t.Errorf("stored ID = %q, want %q", profiles[0].ID, saved.ID)
	}
}

func TestProfileService_SaveProfile_RejectsUnknownID(t *testing.T) {
	// クライアント指定の未知の ID は新規作成として受理しない。
	svc, _ := NewProfileService(newProfileRepo())
	_, err := svc.SaveProfile(domain.BrokerProfile{ID: "../escaped", Name: "Attack"})
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
	if len(svc.GetProfiles()) != 0 {
		t.Error("expected 0 profiles after rejected save")
	}
}

func TestProfileService_SaveProfile_UpdateExisting(t *testing.T) {
	original := domain.BrokerProfile{ID: "p1", Name: "Old", Broker: "tcp://old:1883"}
	svc, _ := NewProfileService(newProfileRepo(original))

	updated := domain.BrokerProfile{ID: "p1", Name: "New", Broker: "tcp://new:1883"}
	saved, err := svc.SaveProfile(updated)
	if err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	if saved.ID != "p1" {
		t.Errorf("ID = %q, want p1", saved.ID)
	}
	profiles := svc.GetProfiles()
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].Name != "New" {
		t.Errorf("Name = %q, want %q", profiles[0].Name, "New")
	}
	if profiles[0].Broker != "tcp://new:1883" {
		t.Errorf("Broker = %q, want %q", profiles[0].Broker, "tcp://new:1883")
	}
}

func TestProfileService_SaveProfile_RepoError(t *testing.T) {
	repo := &inMemoryProfileRepo{
		profiles: map[string]*domain.BrokerProfile{},
		saveErr:  errors.New("write error"),
	}
	svc, _ := NewProfileService(repo)
	// repo まで到達させるため新規作成 (空 ID) で呼ぶ。
	_, err := svc.SaveProfile(domain.BrokerProfile{Name: "X"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProfileService_DeleteProfile_Existing(t *testing.T) {
	repo := newProfileRepo(domain.BrokerProfile{ID: "p1", Name: "ToDelete"})
	svc, _ := NewProfileService(repo)

	if err := svc.DeleteProfile("p1"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	if len(svc.GetProfiles()) != 0 {
		t.Error("expected 0 profiles after delete")
	}
}

func TestProfileService_DeleteProfile_NonExistentID(t *testing.T) {
	// IDが存在しない場合は NotFoundError を返す
	svc, _ := NewProfileService(newProfileRepo())
	err := svc.DeleteProfile("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestProfileService_DeleteProfile_DoesNotCallRepoForMissingID(t *testing.T) {
	// ProfileService は in-memory で存在確認するので、repo.Delete は呼ばれない
	repo := newProfileRepo()
	repo.delErr = errors.New("should not be called")
	svc, _ := NewProfileService(repo)

	err := svc.DeleteProfile("nonexistent")
	// repo.Delete は呼ばれないので、NotFoundError を返す（repo エラーではない）
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError (not repo error), got %T: %v", err, err)
	}
}

func TestProfileService_SaveProfile_GeneratesIDWhenEmpty(t *testing.T) {
	// ID が空の場合は自動採番され、戻り値に入る
	svc, _ := NewProfileService(newProfileRepo())
	saved, err := svc.SaveProfile(domain.BrokerProfile{Name: "NoID", Broker: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	if saved.ID == "" {
		t.Error("expected auto-generated ID, got empty")
	}
}

func TestProfileService_DeleteProfile_RepoError(t *testing.T) {
	repo := newProfileRepo(domain.BrokerProfile{ID: "p1", Name: "X"})
	repo.delErr = errors.New("delete error")
	svc, _ := NewProfileService(repo)

	err := svc.DeleteProfile("p1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProfileService_MultipleProfiles(t *testing.T) {
	svc, _ := NewProfileService(newProfileRepo())
	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		if _, err := svc.SaveProfile(domain.BrokerProfile{Name: name}); err != nil {
			t.Fatalf("SaveProfile(%q): %v", name, err)
		}
	}
	profiles := svc.GetProfiles()
	if len(profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(profiles))
	}
}
