package auction

import (
	"reflect"
	"strings"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCommitmentFormat(t *testing.T) {
	got := Commitment("A1", "bob", 150, "xyz")
	if len(got) != 64 || strings.ToLower(got) != got {
		t.Fatalf("bad commitment %q", got)
	}
	if Commitment("A1", "bob", 150, "xyz") != got {
		t.Fatal("not deterministic")
	}
	if Commitment("A1", "bob", 150, "other") == got {
		t.Fatal("nonce should change commitment")
	}
}

func TestDuplicateCommitRejected(t *testing.T) {
	s := newStore(t)
	if err := s.Create("A1", 100); err != nil {
		t.Fatal(err)
	}
	c := Commitment("A1", "bob", 150, "n1")
	if err := s.Commit("A1", "bob", c); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("A1", "bob", c); err == nil {
		t.Fatal("expected duplicate commit error")
	}
	if err := s.Commit("A1", "bob", "ZZZ"); err == nil {
		t.Fatal("expected invalid commitment error")
	}
}

func TestPhaseEnforcement(t *testing.T) {
	s := newStore(t)
	s.Create("A1", 100)
	if err := s.Reveal("A1", "bob", 150, "n1"); err == nil {
		t.Fatal("reveal before reveal phase should fail")
	}
	if _, err := s.Settle("A1"); err == nil {
		t.Fatal("settle before reveal phase should fail")
	}
	if err := s.OpenReveal("A1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("A1", "bob", Commitment("A1", "bob", 150, "n1")); err == nil {
		t.Fatal("commit after commit phase should fail")
	}
	if err := s.OpenReveal("A1"); err == nil {
		t.Fatal("double phase advance should fail")
	}
}

func TestFailedRevealCanRetry(t *testing.T) {
	s := newStore(t)
	s.Create("A1", 100)
	s.Commit("A1", "bob", Commitment("A1", "bob", 150, "n1"))
	s.OpenReveal("A1")
	if err := s.Reveal("A1", "bob", 150, "wrong"); err == nil {
		t.Fatal("expected mismatch error")
	}
	if err := s.Reveal("A1", "bob", 999, "n1"); err == nil {
		t.Fatal("expected mismatch error")
	}
	if err := s.Reveal("A1", "bob", 150, "n1"); err != nil {
		t.Fatalf("retry with correct values failed: %v", err)
	}
	if err := s.Reveal("A1", "bob", 150, "n1"); err == nil {
		t.Fatal("second successful reveal should fail")
	}
}

func TestTieSettlement(t *testing.T) {
	s := newStore(t)
	s.Create("A1", 100)
	s.Commit("A1", "carol", Commitment("A1", "carol", 200, "c"))
	s.Commit("A1", "alice", Commitment("A1", "alice", 200, "a"))
	s.Commit("A1", "bob", Commitment("A1", "bob", 150, "b"))
	s.OpenReveal("A1")
	s.Reveal("A1", "carol", 200, "c")
	s.Reveal("A1", "alice", 200, "a")
	s.Reveal("A1", "bob", 150, "b")
	res, err := s.Settle("A1")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Sold || res.Winner != "alice" {
		t.Fatalf("tie should go to lexicographically smallest bidder, got %+v", res)
	}
	if res.Price != 200 {
		t.Fatalf("price should be second-highest bid 200, got %d", res.Price)
	}
	again, err := s.Settle("A1")
	if err != nil || !reflect.DeepEqual(again, res) {
		t.Fatal("repeated settle must return same result")
	}
}

func TestSecondPriceAndReserve(t *testing.T) {
	s := newStore(t)
	s.Create("A1", 100)
	s.Commit("A1", "alice", Commitment("A1", "alice", 500, "a"))
	s.Commit("A1", "bob", Commitment("A1", "bob", 80, "b"))
	s.OpenReveal("A1")
	s.Reveal("A1", "alice", 500, "a")
	s.Reveal("A1", "bob", 80, "b")
	res, _ := s.Settle("A1")
	if res.Winner != "alice" || res.Price != 100 {
		t.Fatalf("price should be reserve when second bid below reserve, got %+v", res)
	}
}

func TestNoSaleBelowReserve(t *testing.T) {
	s := newStore(t)
	s.Create("A1", 100)
	s.Commit("A1", "alice", Commitment("A1", "alice", 50, "a"))
	s.OpenReveal("A1")
	s.Reveal("A1", "alice", 50, "a")
	res, _ := s.Settle("A1")
	if res.Sold {
		t.Fatalf("should not sell below reserve: %+v", res)
	}
}

func TestQueryHidesBidsBeforeSettlement(t *testing.T) {
	s := newStore(t)
	s.Create("A1", 100)
	s.Commit("A1", "alice", Commitment("A1", "alice", 500, "a"))
	s.Commit("A1", "bob", Commitment("A1", "bob", 300, "b"))
	s.OpenReveal("A1")
	s.Reveal("A1", "alice", 500, "a")
	v, err := s.Query("A1")
	if err != nil {
		t.Fatal(err)
	}
	if v.Result != nil {
		t.Fatal("result must be hidden before settlement")
	}
	if len(v.Bidders) != 2 || !v.Bidders[0].Revealed || v.Bidders[1].Revealed {
		t.Fatalf("unexpected query view: %+v", v.Bidders)
	}
	res, _ := s.Settle("A1")
	if len(res.Unrevealed) != 1 || res.Unrevealed[0] != "bob" {
		t.Fatalf("unrevealed list wrong: %+v", res.Unrevealed)
	}
	v, _ = s.Query("A1")
	if v.Result == nil || len(v.Result.Bids) != 1 || v.Result.Bids[0].Amount != 500 {
		t.Fatalf("settled query should expose bids: %+v", v)
	}
}

func TestReloadPreservesState(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Create("A1", 100)
	s.Commit("A1", "alice", Commitment("A1", "alice", 500, "a"))
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	v, err := s2.Query("A1")
	if err != nil {
		t.Fatal(err)
	}
	if v.Phase != PhaseCommit || len(v.Bidders) != 1 || v.Bidders[0].Bidder != "alice" {
		t.Fatalf("state not preserved: %+v", v)
	}
	if err := s2.Create("A1", 100); err == nil {
		t.Fatal("duplicate auction id should fail after reload")
	}
}
