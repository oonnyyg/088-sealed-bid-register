package auction

import (
	"errors"
	"sort"
)

func (s *Store) Settle(id string) (*Result, error) {
	a, err := s.get(id)
	if err != nil {
		return nil, err
	}
	if a.Phase == PhaseSettled {
		return a.Result, nil
	}
	if a.Phase != PhaseReveal {
		return nil, errors.New("auction must be in reveal phase to settle")
	}
	a.Result = settle(a)
	a.Phase = PhaseSettled
	return a.Result, nil
}

func settle(a *Auction) *Result {
	res := &Result{Bids: []BidInfo{}, Unrevealed: []string{}}
	for id, b := range a.Bidders {
		if b.Revealed {
			res.Bids = append(res.Bids, BidInfo{Bidder: id, Amount: b.Amount})
		} else {
			res.Unrevealed = append(res.Unrevealed, id)
		}
	}
	sort.Slice(res.Bids, func(i, j int) bool {
		if res.Bids[i].Amount != res.Bids[j].Amount {
			return res.Bids[i].Amount > res.Bids[j].Amount
		}
		return res.Bids[i].Bidder < res.Bids[j].Bidder
	})
	sort.Strings(res.Unrevealed)
	if len(res.Bids) == 0 || res.Bids[0].Amount < a.Reserve {
		return res
	}
	res.Sold = true
	res.Winner = res.Bids[0].Bidder
	res.WinningBid = res.Bids[0].Amount
	res.Price = a.Reserve
	if len(res.Bids) > 1 && res.Bids[1].Amount > res.Price {
		res.Price = res.Bids[1].Amount
	}
	return res
}

type QueryView struct {
	ID      string        `json:"id"`
	Reserve int64         `json:"reserve"`
	Phase   Phase         `json:"phase"`
	Bidders []QueryBidder `json:"bidders"`
	Result  *Result       `json:"result,omitempty"`
}

type QueryBidder struct {
	Bidder     string `json:"bidder"`
	Commitment string `json:"commitment"`
	Revealed   bool   `json:"revealed"`
}

func (s *Store) Query(id string) (*QueryView, error) {
	a, err := s.get(id)
	if err != nil {
		return nil, err
	}
	v := &QueryView{ID: a.ID, Reserve: a.Reserve, Phase: a.Phase, Bidders: []QueryBidder{}}
	ids := make([]string, 0, len(a.Bidders))
	for id := range a.Bidders {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, bid := range ids {
		b := a.Bidders[bid]
		v.Bidders = append(v.Bidders, QueryBidder{Bidder: bid, Commitment: b.Commitment, Revealed: b.Revealed})
	}
	if a.Phase == PhaseSettled {
		v.Result = a.Result
	}
	return v, nil
}
