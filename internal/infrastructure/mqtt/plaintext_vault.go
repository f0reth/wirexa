package mqttinfra

import (
	"fmt"

	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// コンパイル時に domain.PasswordVault を満たすことを検証
var _ domain.PasswordVault = (*PlaintextVault)(nil)

// PlaintextVault は既存の JSON プロファイルからパスワードを読む移行用実装。
// 本番では OS Keychain 実装に差し替える。
type PlaintextVault struct {
	repo *JSONProfileRepository
}

// NewPlaintextVault は PlaintextVault を返す。
func NewPlaintextVault(repo *JSONProfileRepository) *PlaintextVault {
	return &PlaintextVault{repo: repo}
}

// Store は JSON プロファイルへの書き込みを委譲する (現状維持)。
func (v *PlaintextVault) Store(_, _ string) error {
	return nil
}

// Load はプロファイル JSON からパスワードを取得する。
func (v *PlaintextVault) Load(profileID string) (string, error) {
	profiles, err := v.repo.Load()
	if err != nil {
		return "", err
	}
	for _, p := range profiles {
		if p.ID == profileID {
			return p.Password, nil
		}
	}
	return "", fmt.Errorf("profile not found: %s", profileID)
}

// Delete はパスワード削除の no-op 実装。
func (v *PlaintextVault) Delete(_ string) error {
	return nil
}
