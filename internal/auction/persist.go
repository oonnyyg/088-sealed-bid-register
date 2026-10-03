package auction

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const stateFile = "state.json"

// Load reads the store from dir, returning an empty store if none exists.
func Load(dir string) (*Store, error) {
	data, err := os.ReadFile(filepath.Join(dir, stateFile))
	if errors.Is(err, os.ErrNotExist) {
		return NewStore(), nil
	}
	if err != nil {
		return nil, err
	}
	s := NewStore()
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Auctions == nil {
		s.Auctions = map[string]*Auction{}
	}
	return s, nil
}

// Save atomically writes the store to dir.
func (s *Store) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, stateFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, stateFile))
}
