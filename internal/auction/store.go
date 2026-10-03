package auction

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Phase string

const (
	PhaseCommit  Phase = "commit"
	PhaseReveal  Phase = "reveal"
	PhaseSettled Phase = "settled"
)

type Bidder struct {
	Commitment string `json:"commitment"`
	Revealed   bool   `json:"revealed"`
	Amount     int64  `json:"amount,omitempty"`
	Nonce      string `json:"nonce,omitempty"`
}

type Auction struct {
	ID      string            `json:"id"`
	Reserve int64             `json:"reserve"`
	Phase   Phase             `json:"phase"`
	Bidders map[string]*Bidder `json:"bidders"`
	Result  *Result           `json:"result,omitempty"`
}

type Result struct {
	Sold       bool     `json:"sold"`
	Winner     string   `json:"winner,omitempty"`
	Price      int64    `json:"price,omitempty"`
	WinningBid int64    `json:"winning_bid,omitempty"`
	Bids       []BidInfo `json:"bids"`
	Unrevealed []string `json:"unrevealed"`
}

type BidInfo struct {
	Bidder string `json:"bidder"`
	Amount int64  `json:"amount"`
}

type Store struct {
	dir      string
	Auctions map[string]*Auction `json:"auctions"`
}

func Load(dir string) (*Store, error) {
	s := &Store{dir: dir, Auctions: map[string]*Auction{}}
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	s.dir = dir
	if s.Auctions == nil {
		s.Auctions = map[string]*Auction{}
	}
	return s, nil
}

func (s *Store) Save() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, "state.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, "state.json"))
}
