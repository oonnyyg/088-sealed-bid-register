package auction

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
)

type Phase string

const (
	PhaseCommit Phase = "commit"
	PhaseReveal Phase = "reveal"
	PhaseSettle Phase = "settled"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

func validID(s string) bool { return idPattern.MatchString(s) }

var commitmentPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Bid struct {
	Commitment string `json:"commitment"`
	Revealed   bool   `json:"revealed"`
	Amount     int64  `json:"amount,omitempty"`
	Nonce      string `json:"nonce,omitempty"`
}

type Auction struct {
	ID         string          `json:"id"`
	Reserve    int64           `json:"reserve"`
	Phase      Phase           `json:"phase"`
	Bids       map[string]*Bid `json:"bids"`
	Settlement *Settlement     `json:"settlement,omitempty"`
}

type Settlement struct {
	Winner   string        `json:"winner,omitempty"`
	Price    int64         `json:"price,omitempty"`
	Amount   int64         `json:"amount,omitempty"`
	Sold     bool          `json:"sold"`
	Revealed []RevealEntry `json:"revealed"`
	Missing  []string      `json:"missing"`
}

type RevealEntry struct {
	Bidder string `json:"bidder"`
	Amount int64  `json:"amount"`
}

// Commitment computes sha256 hex of "auctionID|bidderID|amount|nonce".
func Commitment(auctionID, bidderID string, amount int64, nonce string) string {
	payload := fmt.Sprintf("%s|%s|%d|%s", auctionID, bidderID, amount, nonce)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

var (
	ErrAuctionExists   = errors.New("auction already exists")
	ErrAuctionNotFound = errors.New("auction not found")
	ErrBadPhase        = errors.New("operation not allowed in current phase")
	ErrDuplicateCommit = errors.New("bidder already committed")
	ErrAlreadyRevealed = errors.New("bid already revealed")
	ErrCommitMismatch  = errors.New("reveal does not match commitment")
	ErrInvalidID       = errors.New("id must be non-empty ASCII letters or digits")
	ErrInvalidReserve  = errors.New("reserve must be a non-negative integer")
	ErrInvalidCommit   = errors.New("commitment must be lowercase sha256 hex")
	ErrInvalidAmount   = errors.New("amount must be a non-negative integer")
)

type Store struct {
	Auctions map[string]*Auction `json:"auctions"`
}

func NewStore() *Store {
	return &Store{Auctions: map[string]*Auction{}}
}

func (s *Store) CreateAuction(id string, reserve int64) error {
	if !validID(id) {
		return ErrInvalidID
	}
	if reserve < 0 {
		return ErrInvalidReserve
	}
	if _, ok := s.Auctions[id]; ok {
		return ErrAuctionExists
	}
	s.Auctions[id] = &Auction{
		ID: id, Reserve: reserve, Phase: PhaseCommit, Bids: map[string]*Bid{},
	}
	return nil
}

func (s *Store) get(id string) (*Auction, error) {
	a, ok := s.Auctions[id]
	if !ok {
		return nil, ErrAuctionNotFound
	}
	return a, nil
}

func (s *Store) Advance(id string) error {
	a, err := s.get(id)
	if err != nil {
		return err
	}
	switch a.Phase {
	case PhaseCommit:
		a.Phase = PhaseReveal
	case PhaseReveal:
		a.Phase = PhaseSettle
	default:
		return ErrBadPhase
	}
	return nil
}

func (s *Store) Commit(auctionID, bidderID, commitment string) error {
	a, err := s.get(auctionID)
	if err != nil {
		return err
	}
	if a.Phase != PhaseCommit {
		return ErrBadPhase
	}
	if !validID(bidderID) {
		return ErrInvalidID
	}
	if !commitmentPattern.MatchString(commitment) {
		return ErrInvalidCommit
	}
	if _, ok := a.Bids[bidderID]; ok {
		return ErrDuplicateCommit
	}
	a.Bids[bidderID] = &Bid{Commitment: commitment}
	return nil
}

func (s *Store) Reveal(auctionID, bidderID string, amount int64, nonce string) error {
	a, err := s.get(auctionID)
	if err != nil {
		return err
	}
	if a.Phase != PhaseReveal {
		return ErrBadPhase
	}
	if !validID(bidderID) || !validID(nonce) {
		return ErrInvalidID
	}
	if amount < 0 {
		return ErrInvalidAmount
	}
	b, ok := a.Bids[bidderID]
	if !ok {
		return ErrAuctionNotFound
	}
	if b.Revealed {
		return ErrAlreadyRevealed
	}
	if Commitment(auctionID, bidderID, amount, nonce) != b.Commitment {
		return ErrCommitMismatch
	}
	b.Revealed = true
	b.Amount = amount
	b.Nonce = nonce
	return nil
}

type QueryBid struct {
	Bidder     string `json:"bidder"`
	Commitment string `json:"commitment"`
	Revealed   bool   `json:"revealed"`
	Amount     *int64 `json:"amount,omitempty"`
}

type QueryResult struct {
	ID         string      `json:"id"`
	Reserve    int64       `json:"reserve"`
	Phase      Phase       `json:"phase"`
	Bids       []QueryBid  `json:"bids"`
	Settlement *Settlement `json:"settlement,omitempty"`
}

func (s *Store) Query(id string) (*QueryResult, error) {
	a, err := s.get(id)
	if err != nil {
		return nil, err
	}
	res := &QueryResult{ID: a.ID, Reserve: a.Reserve, Phase: a.Phase, Settlement: a.Settlement}
	settled := a.Phase == PhaseSettle && a.Settlement != nil
	bidders := make([]string, 0, len(a.Bids))
	for b := range a.Bids {
		bidders = append(bidders, b)
	}
	sort.Strings(bidders)
	for _, name := range bidders {
		b := a.Bids[name]
		qb := QueryBid{Bidder: name, Commitment: b.Commitment, Revealed: b.Revealed}
		if settled && b.Revealed {
			amt := b.Amount
			qb.Amount = &amt
		}
		res.Bids = append(res.Bids, qb)
	}
	return res, nil
}

func (s *Store) Settle(id string) (*Settlement, error) {
	a, err := s.get(id)
	if err != nil {
		return nil, err
	}
	if a.Phase != PhaseSettle {
		return nil, ErrBadPhase
	}
	if a.Settlement != nil {
		return a.Settlement, nil
	}
	st := &Settlement{}
	type entry struct {
		bidder string
		amount int64
	}
	var valid []entry
	bidders := make([]string, 0, len(a.Bids))
	for b := range a.Bids {
		bidders = append(bidders, b)
	}
	sort.Strings(bidders)
	for _, name := range bidders {
		b := a.Bids[name]
		if b.Revealed {
			valid = append(valid, entry{name, b.Amount})
			st.Revealed = append(st.Revealed, RevealEntry{Bidder: name, Amount: b.Amount})
		} else {
			st.Missing = append(st.Missing, name)
		}
	}
	best, second := int64(-1), int64(-1)
	winner := ""
	for _, e := range valid {
		if e.amount > best || (e.amount == best && e.bidder < winner) {
			second = best
			best = e.amount
			winner = e.bidder
		} else if e.amount > second {
			second = e.amount
		}
	}
	if winner != "" && best >= a.Reserve {
		st.Sold = true
		st.Winner = winner
		st.Amount = best
		price := a.Reserve
		if second > price {
			price = second
		}
		st.Price = price
	}
	a.Settlement = st
	return st, nil
}

// ParseAmount validates a decimal string without leading zeros.
func ParseAmount(s string) (int64, error) {
	if s == "" {
		return 0, ErrInvalidAmount
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, ErrInvalidAmount
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, ErrInvalidAmount
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	return n, nil
}
