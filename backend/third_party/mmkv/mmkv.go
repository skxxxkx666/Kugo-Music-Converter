package mmkv

import "errors"

// Vault is the minimal interface required by unlock-music.dev/cli.
type Vault interface {
	Keys() []string
	GetBytes(key string) ([]byte, error)
}

type Manager struct {
	dir string
}

func NewManager(dir string) (*Manager, error) {
	if dir == "" {
		return nil, errors.New("mmkv dir is required")
	}
	return &Manager{dir: dir}, nil
}

func (m *Manager) OpenVault(name string) (Vault, error) {
	if name == "" {
		return nil, errors.New("vault name is required")
	}
	return &noopVault{}, nil
}

func (m *Manager) OpenVaultCrypto(name string, _ string) (Vault, error) {
	if name == "" {
		return nil, errors.New("vault name is required")
	}
	return &noopVault{}, nil
}

type noopVault struct{}

func (v *noopVault) Keys() []string {
	return []string{}
}

func (v *noopVault) GetBytes(_ string) ([]byte, error) {
	return nil, errors.New("mmkv key not found")
}
