package auction

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
)

func validToken(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range []byte(s) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func validCommitment(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range []byte(s) {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func Commitment(auctionID, bidderID string, amount int64, nonce string) string {
	payload := auctionID + "|" + bidderID + "|" + strconv.FormatInt(amount, 10) + "|" + nonce
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (s *Store) Create(id string, reserve int64) error {
	if !validToken(id) {
		return errors.New("invalid auction id")
	}
	if reserve < 0 {
		return errors.New("reserve must be non-negative")
	}
	if _, ok := s.Auctions[id]; ok {
		return fmt.Errorf("auction %q already exists", id)
	}
	s.Auctions[id] = &Auction{ID: id, Reserve: reserve, Phase: PhaseCommit, Bidders: map[string]*Bidder{}}
	return nil
}

func (s *Store) get(id string) (*Auction, error) {
	a, ok := s.Auctions[id]
	if !ok {
		return nil, fmt.Errorf("auction %q not found", id)
	}
	return a, nil
}

func (s *Store) OpenReveal(id string) error {
	a, err := s.get(id)
	if err != nil {
		return err
	}
	if a.Phase != PhaseCommit {
		return fmt.Errorf("auction %q is in %s phase, cannot open reveal", id, a.Phase)
	}
	a.Phase = PhaseReveal
	return nil
}

func (s *Store) Commit(id, bidder, commitment string) error {
	a, err := s.get(id)
	if err != nil {
		return err
	}
	if a.Phase != PhaseCommit {
		return fmt.Errorf("auction %q is in %s phase, cannot commit", id, a.Phase)
	}
	if !validToken(bidder) {
		return errors.New("invalid bidder id")
	}
	if !validCommitment(commitment) {
		return errors.New("commitment must be 64 lowercase hex chars")
	}
	if _, ok := a.Bidders[bidder]; ok {
		return fmt.Errorf("bidder %q already committed", bidder)
	}
	a.Bidders[bidder] = &Bidder{Commitment: commitment}
	return nil
}

func (s *Store) Reveal(id, bidder string, amount int64, nonce string) error {
	a, err := s.get(id)
	if err != nil {
		return err
	}
	if a.Phase != PhaseReveal {
		return fmt.Errorf("auction %q is in %s phase, cannot reveal", id, a.Phase)
	}
	if !validToken(nonce) {
		return errors.New("invalid nonce")
	}
	if amount < 0 {
		return errors.New("amount must be non-negative")
	}
	b, ok := a.Bidders[bidder]
	if !ok {
		return fmt.Errorf("bidder %q has no commitment", bidder)
	}
	if b.Revealed {
		return fmt.Errorf("bidder %q already revealed", bidder)
	}
	if Commitment(id, bidder, amount, nonce) != b.Commitment {
		return errors.New("commitment mismatch")
	}
	b.Revealed = true
	b.Amount = amount
	b.Nonce = nonce
	return nil
}
