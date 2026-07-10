package auth

import (
	"sync"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// MemoryStore is a Store that keeps credentials in process memory only.
// It backs tests and is a building block for environments without an OS
// keyring; nothing is ever written to disk.
type MemoryStore struct {
	mu    sync.RWMutex
	creds map[types.Provider]Credential
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{creds: make(map[types.Provider]Credential)}
}

// Get implements Store.
func (s *MemoryStore) Get(provider types.Provider) (Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cred, ok := s.creds[provider]
	if !ok {
		return Credential{}, ErrNotFound
	}
	return cred, nil
}

// Set implements Store.
func (s *MemoryStore) Set(credential Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.creds[credential.Provider] = credential
	return nil
}

// Delete implements Store.
func (s *MemoryStore) Delete(provider types.Provider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.creds[provider]; !ok {
		return ErrNotFound
	}
	delete(s.creds, provider)
	return nil
}
