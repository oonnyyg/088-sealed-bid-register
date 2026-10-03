package auction

import (
	"errors"
	"testing"
)

func commitBid(t *testing.T, s *Store, auction, bidder string, amount int64, nonce string) {
	t.Helper()
	if err := s.Commit(auction, bidder, Commitment(auction, bidder, amount, nonce)); err != nil {
		t.Fatalf("commit %s: %v", bidder, err)
	}
}

func TestSettleTieBreaksByBidderID(t *testing.T) {
	s := NewStore()
	if err := s.CreateAuction("A1", 100); err != nil {
		t.Fatal(err)
	}
	commitBid(t, s, "A1", "B2", 500, "n2")
	commitBid(t, s, "A1", "B1", 500, "n1")
	commitBid(t, s, "A1", "B3", 300, "n3")
	if err := s.Advance("A1"); err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct {
		b string
		a int64
		n string
	}{
		{"B2", 500, "n2"}, {"B1", 500, "n1"}, {"B3", 300, "n3"},
	} {
		if err := s.Reveal("A1", r.b, r.a, r.n); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Advance("A1"); err != nil {
		t.Fatal(err)
	}
	st, err := s.Settle("A1")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Sold || st.Winner != "B1" || st.Amount != 500 || st.Price != 500 {
		t.Fatalf("unexpected settlement: %+v", st)
	}
	again, err := s.Settle("A1")
	if err != nil {
		t.Fatal(err)
	}
	if again.Winner != st.Winner || again.Price != st.Price || again.Amount != st.Amount || again.Sold != st.Sold {
		t.Fatalf("re-settle differs: %+v vs %+v", again, st)
	}
}

func TestFailedRevealDoesNotConsumeRetry(t *testing.T) {
	s := NewStore()
	if err := s.CreateAuction("A1", 0); err != nil {
		t.Fatal(err)
	}
	commitBid(t, s, "A1", "B1", 250, "good")
	if err := s.Advance("A1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Reveal("A1", "B1", 250, "wrong"); !errors.Is(err, ErrCommitMismatch) {
		t.Fatalf("want mismatch, got %v", err)
	}
	if err := s.Reveal("A1", "B1", 999, "good"); !errors.Is(err, ErrCommitMismatch) {
		t.Fatalf("want mismatch, got %v", err)
	}
	if err := s.Reveal("A1", "B1", 250, "good"); err != nil {
		t.Fatalf("retry with correct values failed: %v", err)
	}
	if err := s.Reveal("A1", "B1", 250, "good"); !errors.Is(err, ErrAlreadyRevealed) {
		t.Fatalf("want already revealed, got %v", err)
	}
}

func TestCommitRulesAndPhases(t *testing.T) {
	s := NewStore()
	if err := s.CreateAuction("A1", -1); !errors.Is(err, ErrInvalidReserve) {
		t.Fatalf("want invalid reserve, got %v", err)
	}
	if err := s.CreateAuction("A1", 10); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateAuction("A1", 10); !errors.Is(err, ErrAuctionExists) {
		t.Fatalf("want exists, got %v", err)
	}
	commitBid(t, s, "A1", "B1", 10, "x")
	if err := s.Commit("A1", "B1", Commitment("A1", "B1", 20, "y")); !errors.Is(err, ErrDuplicateCommit) {
		t.Fatalf("want duplicate, got %v", err)
	}
	if err := s.Reveal("A1", "B1", 10, "x"); !errors.Is(err, ErrBadPhase) {
		t.Fatalf("want bad phase, got %v", err)
	}
	if _, err := s.Settle("A1"); !errors.Is(err, ErrBadPhase) {
		t.Fatalf("want bad phase, got %v", err)
	}
	if err := s.Advance("A1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("A1", "B2", Commitment("A1", "B2", 5, "z")); !errors.Is(err, ErrBadPhase) {
		t.Fatalf("want bad phase, got %v", err)
	}
}

func TestNoSaleAndReserveFloor(t *testing.T) {
	s := NewStore()
	if err := s.CreateAuction("A1", 400); err != nil {
		t.Fatal(err)
	}
	commitBid(t, s, "A1", "B1", 300, "n1")
	commitBid(t, s, "A1", "B2", 500, "n2")
	commitBid(t, s, "A1", "B3", 600, "n3")
	s.Advance("A1")
	if err := s.Reveal("A1", "B1", 300, "n1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Reveal("A1", "B2", 500, "n2"); err != nil {
		t.Fatal(err)
	}
	// B3 never reveals.
	s.Advance("A1")
	st, err := s.Settle("A1")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Sold || st.Winner != "B2" || st.Price != 400 {
		t.Fatalf("unexpected settlement: %+v", st)
	}
	if len(st.Missing) != 1 || st.Missing[0] != "B3" {
		t.Fatalf("missing list: %+v", st.Missing)
	}

	s2 := NewStore()
	if err := s2.CreateAuction("A2", 1000); err != nil {
		t.Fatal(err)
	}
	commitBid(t, s2, "A2", "B1", 500, "n1")
	s2.Advance("A2")
	s2.Reveal("A2", "B1", 500, "n1")
	s2.Advance("A2")
	st2, err := s2.Settle("A2")
	if err != nil {
		t.Fatal(err)
	}
	if st2.Sold {
		t.Fatalf("expected no sale, got %+v", st2)
	}
}

func TestQueryHidesBidBeforeSettlement(t *testing.T) {
	s := NewStore()
	if err := s.CreateAuction("A1", 0); err != nil {
		t.Fatal(err)
	}
	commitBid(t, s, "A1", "B1", 100, "n1")
	s.Advance("A1")
	s.Reveal("A1", "B1", 100, "n1")
	q, err := s.Query("A1")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Bids) != 1 || !q.Bids[0].Revealed || q.Bids[0].Amount != nil {
		t.Fatalf("pre-settlement query leaks amount: %+v", q.Bids)
	}
	s.Advance("A1")
	if _, err := s.Settle("A1"); err != nil {
		t.Fatal(err)
	}
	q, err = s.Query("A1")
	if err != nil {
		t.Fatal(err)
	}
	if q.Bids[0].Amount == nil || *q.Bids[0].Amount != 100 {
		t.Fatalf("post-settlement query hides amount: %+v", q.Bids)
	}
}

func TestPersistenceAcrossReload(t *testing.T) {
	dir := t.TempDir()
	s := NewStore()
	if err := s.CreateAuction("A1", 50); err != nil {
		t.Fatal(err)
	}
	commitBid(t, s, "A1", "B1", 100, "n1")
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}

	s2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	q, err := s2.Query("A1")
	if err != nil {
		t.Fatal(err)
	}
	if q.Phase != PhaseCommit || len(q.Bids) != 1 || q.Bids[0].Bidder != "B1" {
		t.Fatalf("reloaded state mismatch: %+v", q)
	}
	if err := s2.CreateAuction("A1", 50); !errors.Is(err, ErrAuctionExists) {
		t.Fatalf("want exists after reload, got %v", err)
	}
}

func TestParseAmount(t *testing.T) {
	for _, bad := range []string{"", "01", "-1", "1.5", "abc"} {
		if _, err := ParseAmount(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
	for _, good := range []string{"0", "7", "123456"} {
		if _, err := ParseAmount(good); err != nil {
			t.Fatalf("unexpected error for %q: %v", good, err)
		}
	}
}
