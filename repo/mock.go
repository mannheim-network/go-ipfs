package repo

import (
	"errors"

	filestore "github.com/ipfs/go-ipfs/filestore"
	keystore "github.com/ipfs/go-ipfs/keystore"

	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	config "gx/ipfs/QmXctaABKwgzmQgNM4bucMJf7zJnxxvhmPM1Pw95dxUfB5/go-ipfs-config"
)

var errTODO = errors.New("TODO: mock repo")

// Mock is not thread-safe
type Mock struct {
	C config.Config
	D Datastore
	K keystore.Keystore
}

func (m *Mock) Config() (*config.Config, error) {
	return &m.C, nil // FIXME threadsafety
}

func (m *Mock) SetConfig(updated *config.Config) error {
	m.C = *updated // FIXME threadsafety
	return nil
}

func (m *Mock) BackupConfig(prefix string) (string, error) {
	return "", errTODO
}

func (m *Mock) SetConfigKey(key string, value interface{}) error {
	return errTODO
}

func (m *Mock) GetConfigKey(key string) (interface{}, error) {
	return nil, errTODO
}

func (m *Mock) Datastore() Datastore { return m.D }

func (m *Mock) GetStorageUsage() (uint64, error) { return 0, nil }

func (m *Mock) Close() error { return errTODO }

func (m *Mock) SetAPIAddr(addr ma.Multiaddr) error { return errTODO }

func (m *Mock) Keystore() keystore.Keystore { return m.K }

func (m *Mock) SwarmKey() ([]byte, error) {
	return nil, nil
}

func (m *Mock) FileManager() *filestore.FileManager { return nil }
