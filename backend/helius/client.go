package helius

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"sentinelbag/models"
)

const (
	restBaseURL = "https://api.helius.xyz/v0"
	dasBaseURL  = "https://mainnet.helius-rpc.com"
	pageSize    = 100 // max per Helius enhanced-transactions page

	// Mint-direct: cursor-based pagination, sequential within Stage 1 goroutine.
	// Stage 1 and Stage 2 run concurrently so this doesn't block the holder fan-out.
	// Early-exit fires when targetTxs is reached, so large tokens stop early.
	mintPages = 10

	// Holder fan-out: 1 page per holder; concurrency across holders compensates.
	// holderSampleDeep is used for non-pump tokens that need broader wallet coverage.
	holderPages      = 1
	holderSample     = 15 // pump.fun / small-cap tokens
	holderSampleDeep = 25 // all other tokens

	targetTxs = 1000

	// MinSampleSize is exported so the handler can gate analysis on it.
	// 150 transactions is the minimum for statistically reliable heuristic scores,
	// especially in strict-mode small-cap analysis.
	MinSampleSize = 100
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 12 * time.Second},
	}
}

// GetTransactionsForToken fetches on-chain transfer history for a token mint.
//
// Both strategies run concurrently in separate goroutines:
//
//  1. Mint-direct: GET /v0/addresses/{mint}/transactions with no type filter,
//     paginated with the `before` cursor (sequential within this goroutine).
//     mintPages × pageSize raw transactions — captures SWAP + TRANSFER + all types.
//
//  2. Holder fan-out: DAS getTokenAccounts → top-N owner wallets → concurrent
//     per-holder requests (TRANSFER only, 1 page each).
//
// Results from both stages are merged and deduplicated by signature.
// Callers should verify len(result) >= MinSampleSize before analysis.
func (c *Client) GetTransactionsForToken(ctx context.Context, tokenMint string) ([]models.HeliusTransaction, error) {
	prefix := tokenMint
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}

	type stageResult struct {
		txns []models.HeliusTransaction
		err  error
	}

	mintCh := make(chan stageResult, 1)
	holderCh := make(chan stageResult, 1)

	// ── Stage 1: mint-direct ────────────────────────────────────────────────
	go func() {
		txns, err := c.fetchMintTransactions(ctx, tokenMint)
		mintCh <- stageResult{txns, err}
	}()

	// ── Stage 2: holder fan-out ─────────────────────────────────────────────
	// Deeper sample for non-pump tokens (larger tokens need more holder coverage).
	sample := holderSample
	if !strings.HasSuffix(strings.ToLower(tokenMint), "pump") {
		sample = holderSampleDeep
	}
	go func() {
		holders, err := c.getTopHolders(ctx, tokenMint, sample)
		if err != nil || len(holders) == 0 {
			holderCh <- stageResult{nil, err}
			return
		}
		holderCh <- stageResult{c.fetchHolderTransactions(ctx, holders, tokenMint), nil}
	}()

	// Collect both stages (wait for whichever finishes last)
	mintRes := <-mintCh
	holderRes := <-holderCh

	log.Printf("helius: mint-direct=%d holder-fanout=%d for %s…",
		len(mintRes.txns), len(holderRes.txns), prefix)

	// Merge: mint-direct is preferred (primary); holders fill the gap
	seen := map[string]bool{}
	all := make([]models.HeliusTransaction, 0)

	for _, tx := range mintRes.txns {
		if !seen[tx.Signature] {
			seen[tx.Signature] = true
			all = append(all, tx)
		}
	}
	for _, tx := range holderRes.txns {
		if !seen[tx.Signature] && len(all) < targetTxs {
			seen[tx.Signature] = true
			all = append(all, tx)
		}
	}

	log.Printf("helius: total %d unique transactions for %s…", len(all), prefix)

	if len(all) == 0 {
		if mintRes.err != nil {
			return nil, mintRes.err
		}
		return nil, fmt.Errorf("helius: no transactions found for %s", tokenMint)
	}
	if len(all) > targetTxs {
		all = all[:targetTxs]
	}
	return all, nil
}

// fetchMintTransactions queries the mint address directly, paginating with the
// `before` cursor. No type filter — captures SWAP, TRANSFER, and all other types.
func (c *Client) fetchMintTransactions(ctx context.Context, tokenMint string) ([]models.HeliusTransaction, error) {
	var relevant []models.HeliusTransaction
	cursor := ""

	for p := 0; p < mintPages; p++ {
		pageURL := fmt.Sprintf(
			"%s/addresses/%s/transactions?api-key=%s&limit=%d",
			restBaseURL, tokenMint, c.apiKey, pageSize,
		)
		if cursor != "" {
			pageURL += "&before=" + cursor
		}

		rawPage, err := c.fetchPage(ctx, pageURL)
		if err != nil {
			if p == 0 {
				return nil, err
			}
			break
		}
		if len(rawPage) == 0 {
			break
		}

		before := len(relevant)
		for _, tx := range rawPage {
			for _, tt := range tx.TokenTransfers {
				if tt.Mint == tokenMint {
					relevant = append(relevant, tx)
					break
				}
			}
		}
		log.Printf("helius: mint page %d — %d raw / %d matched (cursor=%s)",
			p+1, len(rawPage), len(relevant)-before, cursor)

		if len(relevant) >= targetTxs || len(rawPage) < pageSize {
			break
		}
		cursor = rawPage[len(rawPage)-1].Signature
	}

	return relevant, nil
}

// fetchHolderTransactions queries each holder's TRANSFER history concurrently
// (holderPages pages each) and returns the merged results.
func (c *Client) fetchHolderTransactions(ctx context.Context, holders []string, tokenMint string) []models.HeliusTransaction {
	resultCh := make(chan []models.HeliusTransaction, len(holders))
	var wg sync.WaitGroup

	for _, addr := range holders {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			txns, _ := c.fetchAddressTransactions(ctx, a, tokenMint)
			resultCh <- txns
		}(addr)
	}
	go func() { wg.Wait(); close(resultCh) }()

	var all []models.HeliusTransaction
	for txns := range resultCh {
		all = append(all, txns...)
	}
	return all
}

// getTopHolders queries the Helius DAS getTokenAccounts method and returns
// deduplicated owner wallet addresses for the top holders of the token.
// sampleSize controls how many distinct owner wallets to return.
func (c *Client) getTopHolders(ctx context.Context, mint string, sampleSize int) ([]string, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      "sentinel-holders",
		"method":  "getTokenAccounts",
		"params": map[string]any{
			"mint":  mint,
			"limit": 20,
			"page":  1,
		},
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/?api-key=%s", dasBaseURL, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("helius DAS: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("helius DAS: request: %w", err)
	}
	defer resp.Body.Close()

	var das models.DASResponse
	if err := json.NewDecoder(resp.Body).Decode(&das); err != nil {
		return nil, fmt.Errorf("helius DAS: decode: %w", err)
	}
	if das.Error != nil {
		return nil, fmt.Errorf("helius DAS: %s", das.Error.Message)
	}

	seen := map[string]bool{}
	var owners []string
	for _, acc := range das.Result.TokenAccounts {
		if acc.Owner != "" && !seen[acc.Owner] {
			seen[acc.Owner] = true
			owners = append(owners, acc.Owner)
			if len(owners) >= sampleSize {
				break
			}
		}
	}
	return owners, nil
}

// fetchAddressTransactions paginates up to holderPages of TRANSFER transactions
// for walletAddr, returning only those that include a transfer of tokenMint.
func (c *Client) fetchAddressTransactions(ctx context.Context, walletAddr, tokenMint string) ([]models.HeliusTransaction, error) {
	var relevant []models.HeliusTransaction
	cursor := ""

	for p := 0; p < holderPages; p++ {
		pageURL := fmt.Sprintf(
			"%s/addresses/%s/transactions?api-key=%s&limit=%d&type=TRANSFER",
			restBaseURL, walletAddr, c.apiKey, pageSize,
		)
		if cursor != "" {
			pageURL += "&before=" + cursor
		}

		rawPage, err := c.fetchPage(ctx, pageURL)
		if err != nil {
			if p == 0 {
				return nil, err
			}
			break
		}
		if len(rawPage) == 0 {
			break
		}

		for _, tx := range rawPage {
			for _, tt := range tx.TokenTransfers {
				if tt.Mint == tokenMint {
					relevant = append(relevant, tx)
					break
				}
			}
		}

		if len(rawPage) < pageSize {
			break
		}
		cursor = rawPage[len(rawPage)-1].Signature
	}

	return relevant, nil
}

// fetchPage performs a single GET to url with up to 2 attempts and backoff on 429.
func (c *Client) fetchPage(ctx context.Context, url string) ([]models.HeliusTransaction, error) {
	var lastErr error
	backoff := 1 * time.Second

	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("helius: build request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("helius: http: %w", err)
			continue
		}

		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("helius: rate limited (429)")
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("helius: address not found (404)")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("helius: status %d: %s", resp.StatusCode, truncate(string(raw), 200))
		}

		var page []models.HeliusTransaction
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("helius: decode: %w", err)
		}
		return page, nil
	}
	return nil, lastErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// IsRateLimit reports whether err indicates a Helius 429 response.
func IsRateLimit(err error) bool {
	return err != nil && strings.Contains(err.Error(), "rate limited")
}

// IsNotFound reports whether err indicates a Helius 404 response.
func IsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}
