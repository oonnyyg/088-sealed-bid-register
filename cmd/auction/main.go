package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"example.com/sealed-bid-register/internal/auction"
)

type command struct {
	Cmd        string `json:"cmd"`
	Auction    string `json:"auction"`
	Reserve    int64  `json:"reserve"`
	Bidder     string `json:"bidder"`
	Commitment string `json:"commitment"`
	Amount     int64  `json:"amount"`
	Nonce      string `json:"nonce"`
}

func reply(v any, err error) {
	out := map[string]any{}
	if err != nil {
		out["ok"] = false
		out["error"] = err.Error()
	} else {
		out["ok"] = true
		if v != nil {
			out["data"] = v
		}
	}
	data, _ := json.Marshal(out)
	fmt.Println(string(data))
}

func main() {
	stateDir := flag.String("state", "", "state directory")
	flag.Parse()
	if *stateDir == "" {
		fmt.Fprintln(os.Stderr, "missing --state directory")
		os.Exit(2)
	}
	store, err := auction.Load(*stateDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var c command
		if err := json.Unmarshal(line, &c); err != nil {
			reply(nil, fmt.Errorf("invalid JSON: %v", err))
			continue
		}
		var (
			data any
			err  error
		)
		switch c.Cmd {
		case "create":
			err = store.Create(c.Auction, c.Reserve)
		case "open_reveal":
			err = store.OpenReveal(c.Auction)
		case "commit":
			err = store.Commit(c.Auction, c.Bidder, c.Commitment)
		case "reveal":
			err = store.Reveal(c.Auction, c.Bidder, c.Amount, c.Nonce)
		case "query":
			data, err = store.Query(c.Auction)
		case "settle":
			data, err = store.Settle(c.Auction)
		default:
			err = fmt.Errorf("unknown command %q", c.Cmd)
		}
		if err == nil {
			err = store.Save()
		}
		reply(data, err)
	}
}
