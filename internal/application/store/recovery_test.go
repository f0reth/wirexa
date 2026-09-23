package store

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

// fakeSingleFile は SingleFileRepository のフェイク。Quarantine の呼び出し回数を記録する。
type fakeSingleFile struct {
	loadErr       error
	quarantineErr error
	items         []string
	quarantines   int
}

func (r *fakeSingleFile) Load() ([]string, error) {
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	return r.items, nil
}

func (r *fakeSingleFile) Quarantine() (string, error) {
	r.quarantines++
	if r.quarantineErr != nil {
		return "", r.quarantineErr
	}
	return "dir/file.json.corrupt", nil
}

// countLogger は Error の呼び出し回数だけを数えるテスト用ロガー。
type countLogger struct{ errors int }

func (l *countLogger) Info(_ string, _ ...any)  {}
func (l *countLogger) Debug(_ string, _ ...any) {}
func (l *countLogger) Error(_ string, _ ...any) { l.errors++ }

func TestLoadSingleFile(t *testing.T) {
	corrupt := fmt.Errorf("%w: bad json", cmn.ErrCorruptData)
	readErr := errors.New("permission denied")
	states := []struct {
		repo           fakeSingleFile
		name           string
		wantItems      []string
		wantQuarantine int
		wantLog        bool
		wantErr        bool
	}{
		{name: "正常", repo: fakeSingleFile{items: []string{"a"}}, wantItems: []string{"a"}},
		{name: "破損+退避成功", repo: fakeSingleFile{loadErr: corrupt}, wantQuarantine: 1, wantLog: true},
		{name: "破損+退避失敗", repo: fakeSingleFile{loadErr: corrupt, quarantineErr: errors.New("rename failed")}, wantQuarantine: 1, wantLog: true},
		{name: "読み込み失敗", repo: fakeSingleFile{loadErr: readErr}, wantErr: true},
	}
	// persist の期待値はポリシー × 状態で決まる。退避失敗時だけポリシーで分かれる。
	wantPersist := map[RecoveryPolicy]map[string]bool{
		PolicyBestEffort:  {"正常": true, "破損+退避成功": true, "破損+退避失敗": false, "読み込み失敗": false},
		PolicyRegenerable: {"正常": true, "破損+退避成功": true, "破損+退避失敗": true, "読み込み失敗": false},
	}
	policies := map[string]RecoveryPolicy{"BestEffort": PolicyBestEffort, "Regenerable": PolicyRegenerable}

	for policyName, policy := range policies {
		for _, st := range states {
			t.Run(policyName+"/"+st.name, func(t *testing.T) {
				repo := st.repo
				logger := &countLogger{}

				items, persist, err := LoadSingleFile(&repo, policy, logger, "test")

				if (err != nil) != st.wantErr {
					t.Fatalf("err = %v, wantErr %v", err, st.wantErr)
				}
				if st.wantErr && !errors.Is(err, readErr) {
					t.Errorf("err = %v, want the read error", err)
				}
				if want := wantPersist[policy][st.name]; persist != want {
					t.Errorf("persist = %v, want %v", persist, want)
				}
				if !slices.Equal(items, st.wantItems) {
					t.Errorf("items = %v, want %v", items, st.wantItems)
				}
				if repo.quarantines != st.wantQuarantine {
					t.Errorf("Quarantine calls = %d, want %d", repo.quarantines, st.wantQuarantine)
				}
				if (logger.errors > 0) != st.wantLog {
					t.Errorf("error logs = %d, wantLog %v", logger.errors, st.wantLog)
				}
			})
		}
	}
}

// logger が nil でも破損時の退避で panic しない。
func TestLoadSingleFile_NilLogger(t *testing.T) {
	repo := &fakeSingleFile{loadErr: fmt.Errorf("%w: bad json", cmn.ErrCorruptData)}
	if _, persist, err := LoadSingleFile(repo, PolicyRegenerable, nil, "test"); err != nil || !persist {
		t.Fatalf("LoadSingleFile = (persist %v, err %v), want true, nil", persist, err)
	}
}
