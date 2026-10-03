package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"example.com/sealed-bid-register/internal/auction"
)

type request struct {
	Cmd        string `json:"cmd"`
	Auction    string `json:"auction"`
	Bidder     string `json:"bidder"`
	Commitment string `json:"commitment"`
	Nonce      string `json:"nonce"`
	Reserve    *int64 `json:"reserve"`
	Amount     *int64 `json:"amount"`
}

type response struct {
	OK     bool        `json:"ok"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func main() {
	stateDir := flag.String("state", "", "state directory")
	flag.Parse()
	if *stateDir == "" {
		fmt.Fprintln(os.Stderr, "usage: auction --state DIR")
		os.Exit(2)
	}
	store, err := auction.Load(*stateDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load state:", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		resp := handle(store, *stateDir, line)
		if err := enc.Encode(resp); err != nil {
			fmt.Fprintln(os.Stderr, "encode:", err)
			os.Exit(1)
		}
	}
}

func handle(store *auction.Store, dir string, line []byte) response {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return response{Error: "invalid JSON: " + err.Error()}
	}
	mutate := func(err error) response {
		if err != nil {
			return response{Error: err.Error()}
		}
		if err := store.Save(dir); err != nil {
			return response{Error: "save state: " + err.Error()}
		}
		return response{OK: true}
	}
	switch req.Cmd {
	case "create":
		if req.Reserve == nil {
			return response{Error: "reserve is required"}
		}
		return mutate(store.CreateAuction(req.Auction, *req.Reserve))
	case "advance":
		return mutate(store.Advance(req.Auction))
	case "commit":
		return mutate(store.Commit(req.Auction, req.Bidder, req.Commitment))
	case "reveal":
		if req.Amount == nil {
			return response{Error: "amount is required"}
		}
		return mutate(store.Reveal(req.Auction, req.Bidder, *req.Amount, req.Nonce))
	case "query":
		res, err := store.Query(req.Auction)
		if err != nil {
			return response{Error: err.Error()}
		}
		return response{OK: true, Result: res}
	case "settle":
		st, err := store.Settle(req.Auction)
		if err != nil {
			return response{Error: err.Error()}
		}
		if err := store.Save(dir); err != nil {
			return response{Error: "save state: " + err.Error()}
		}
		return response{OK: true, Result: st}
	default:
		return response{Error: "unknown cmd: " + req.Cmd}
	}
}
