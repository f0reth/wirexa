package mqttinfra

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
	"github.com/f0reth/Wirexa/internal/testutil"
)

func newTestProfileRepo(t *testing.T) (*ProfileRepository, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := NewProfileRepository(dir, nil)
	if err != nil {
		t.Fatalf("NewProfileRepository: %v", err)
	}
	return repo, dir
}

// 全フィールドを埋めたプロファイルを保存・復元しても値を失わない。検査対象は型から決まるので、
// domain 型にフィールドを足して永続化 DTO や変換関数への追加を忘れるとここで検出される。
func TestProfileRepository_RoundTripKeepsEveryField(t *testing.T) {
	var want domain.BrokerProfile
	testutil.Populate(t, &want)

	repo, _ := newTestProfileRepo(t)
	if err := repo.Save(&want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

// 既存の保存形式 (golden) を読めて、書き戻すと同じ JSON になる。
func TestProfileRepository_GoldenFormat(t *testing.T) {
	golden := testutil.ReadGolden(t, filepath.Join("testdata", "profile.golden.json"))
	repo, dir := newTestProfileRepo(t)
	path := filepath.Join(dir, "profile-1.json")
	if err := os.WriteFile(path, golden, 0o600); err != nil {
		t.Fatalf("write golden: %v", err)
	}

	loaded, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := domain.BrokerProfile{
		ID: "profile-1", Name: "Local", Broker: "tcp://localhost:1883", ClientID: "wirexa-1",
		Username: "user", Password: "pass", UseTLS: true,
	}
	if len(loaded) != 1 || loaded[0] != want {
		t.Fatalf("golden load mismatch:\n got %+v\nwant %+v", loaded, want)
	}

	if err = repo.Save(&loaded[0]); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	testutil.AssertJSONEqual(t, saved, golden)
}

// 永続化 DTO はどの階層でも domain 型を埋め込まない。
func TestProfileRepository_StoredDTOHasNoDomainTypes(t *testing.T) {
	testutil.AssertNoTypesFrom(t, storedBrokerProfile{}, testutil.DomainPkg)
}

// 破損ファイルは退避してスキップし、読めたプロファイルだけを返す。
func TestProfileRepository_QuarantinesCorruptFile(t *testing.T) {
	repo, dir := newTestProfileRepo(t)
	if err := repo.Save(&domain.BrokerProfile{ID: "ok", Name: "OK"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	corrupt := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(corrupt, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	got, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].ID != "ok" {
		t.Fatalf("Load = %+v, want only the readable profile", got)
	}
	if _, err := os.Stat(corrupt + ".corrupt"); err != nil {
		t.Fatalf("corrupt file was not quarantined: %v", err)
	}
}

// ストア外を指しうる ID は保存前に拒否する。
func TestProfileRepository_SaveRejectsUnsafeID(t *testing.T) {
	repo, dir := newTestProfileRepo(t)
	for _, id := range []string{"", "../escape", `..\escape`, "con"} {
		if err := repo.Save(&domain.BrokerProfile{ID: id}); err == nil {
			t.Errorf("Save(%q) must fail", id)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.json")); !os.IsNotExist(err) {
		t.Fatalf("file was written outside the store: %v", err)
	}
}
