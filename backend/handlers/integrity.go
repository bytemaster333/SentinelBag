package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sentinelbag/analysis"
	"sentinelbag/cache"
	"sentinelbag/helius"
	"sentinelbag/models"
)

// IntegrityHandler handles wash trading analysis requests.
type IntegrityHandler struct {
	helius *helius.Client
	cache  *cache.Store
}

func NewIntegrityHandler(h *helius.Client, c *cache.Store) *IntegrityHandler {
	return &IntegrityHandler{helius: h, cache: c}
}

// GetIntegrityScore godoc
//
//	@Summary      Analyze a Solana token for wash trading signals
//	@Description  Fetches recent transactions from the token's top holders via Helius DAS,
//	@Description  then runs three deterministic heuristics concurrently:
//	@Description  (1) Wallet Clustering using the Herfindahl–Hirschman Index,
//	@Description  (2) Circular Flow Detection (A→B→A and A→B→C→A patterns),
//	@Description  (3) Buyer Diversity Index (unique recipients / total transfers).
//	@Description  Results are cached in Redis for 1 hour.
//	@Tags         integrity
//	@Accept       json
//	@Produce      json
//	@Param        tokenAddress  path      string  true  "Solana token mint address (base58, 32–44 chars)"  example(DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263)
//	@Success      200  {object}  models.IntegrityScore
//	@Failure      400  {object}  map[string]string  "invalid token address format"
//	@Failure      404  {object}  map[string]string  "token not found or no transactions"
//	@Failure      422  {object}  map[string]string  "insufficient data (< 100 transactions)"
//	@Failure      429  {object}  map[string]string  "Helius API rate limit reached"
//	@Failure      504  {object}  map[string]string  "analysis pipeline timed out"
//	@Router       /api/integrity/{tokenAddress} [get]
func (h *IntegrityHandler) GetIntegrityScore(w http.ResponseWriter, r *http.Request) {
	tokenAddress := chi.URLParam(r, "tokenAddress")
	if len(tokenAddress) < 32 || len(tokenAddress) > 44 {
		writeError(w, http.StatusBadRequest, "invalid token address: must be a base58 Solana address (32–44 chars)")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Cache hit — patch Cached flag and return immediately
	if cached, ok := h.cache.Get(ctx, tokenAddress); ok {
		var resp models.IntegrityScore
		if err := json.Unmarshal(cached, &resp); err == nil {
			resp.Cached = true
			writeJSON(w, http.StatusOK, resp)
			return
		}
	}

	// Fetch transactions via DAS holder fan-out
	txns, err := h.helius.GetTransactionsForToken(ctx, tokenAddress)
	if err != nil {
		switch {
		case helius.IsRateLimit(err):
			writeError(w, http.StatusTooManyRequests, "Helius rate limit reached — please retry in a moment")
		case helius.IsNotFound(err):
			writeError(w, http.StatusNotFound, "token address not found on-chain")
		default:
			writeError(w, http.StatusInternalServerError, "failed to fetch transactions: "+err.Error())
		}
		return
	}
	if len(txns) == 0 {
		writeError(w, http.StatusNotFound, "no transactions found for this token — try a more active token address")
		return
	}
	if len(txns) < helius.MinSampleSize {
		writeError(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("insufficient data: only %d transactions found (minimum %d required for a reliable analysis — token may be inactive or newly launched)",
				len(txns), helius.MinSampleSize))
		return
	}

	// Proof of Ecosystem: compute infraShare and uniqueSenders once; pass to all
	// three heuristics so they share the same authoritative tier classification.
	// uniqueSenders is used (not len(txns)) — a wash trader can inflate txn count
	// trivially, but creating 400+ distinct funded wallets is prohibitively expensive.
	infraShare := analysis.ComputeInfraShare(txns, tokenAddress)
	uniqueSenders := analysis.ComputeUniqueSenders(txns, tokenAddress)
	log.Printf("integrity: analysing %d transactions for %s — infraShare=%.1f%% uniqueSenders=%d tier=%s",
		len(txns), tokenAddress, infraShare*100, uniqueSenders,
		analysis.TierLabel(analysis.ClassifyTier(tokenAddress, uniqueSenders, infraShare)))

	// Fan-out: run all three heuristics concurrently on the same read-only slice
	resultCh := make(chan models.AnalysisResult, 3)
	go func() { resultCh <- analysis.AnalyzeClustering(txns, tokenAddress, infraShare, uniqueSenders) }()
	go func() { resultCh <- analysis.AnalyzeCircular(txns, tokenAddress, infraShare, uniqueSenders) }()
	go func() { resultCh <- analysis.AnalyzeDiversity(txns, tokenAddress, infraShare, uniqueSenders) }()

	evidence := make([]models.AnalysisResult, 0, 3)
	for i := 0; i < 3; i++ {
		select {
		case res := <-resultCh:
			evidence = append(evidence, res)
		case <-ctx.Done():
			writeError(w, http.StatusGatewayTimeout, "analysis pipeline timed out after 30s")
			return
		}
	}

	// Ensure rule ordering is deterministic for consistent caching
	sortEvidence(evidence)

	totalPenalty := 0
	var flags []string
	for _, e := range evidence {
		totalPenalty += e.Score
		if e.Flag != "" {
			flags = append(flags, e.Flag)
		}
	}
	if flags == nil {
		flags = []string{}
	}

	score := clamp(100-totalPenalty, 0, 100)
	grade := gradeFromScore(score)
	log.Printf("integrity: score=%d grade=%s flags=%v txns=%d token=%s",
		score, grade, flags, len(txns), tokenAddress)

	resp := models.IntegrityScore{
		Token:      tokenAddress,
		Score:      score,
		Grade:      grade,
		Flags:      flags,
		Evidence:   evidence,
		SampleSize: len(txns),
		Cached:     false,
	}

	// Cache asynchronously — does not block the response
	go func() {
		if data, err := json.Marshal(resp); err == nil {
			storeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			h.cache.Set(storeCtx, tokenAddress, data)
		}
	}()

	writeJSON(w, http.StatusOK, resp)
}

// sortEvidence ensures Wallet Clustering, Circular Flow, Buyer Diversity always appear
// in the same order regardless of goroutine completion order.
func sortEvidence(ev []models.AnalysisResult) {
	order := map[string]int{
		"Wallet Clustering": 0,
		"Circular Flow":     1,
		"Buyer Diversity":   2,
	}
	for i := 0; i < len(ev); i++ {
		for j := i + 1; j < len(ev); j++ {
			if order[ev[i].Rule] > order[ev[j].Rule] {
				ev[i], ev[j] = ev[j], ev[i]
			}
		}
	}
}

func gradeFromScore(score int) string {
	switch {
	case score >= 90:
		return "A+"
	case score >= 80:
		return "A"
	case score >= 70:
		return "B"
	case score >= 50:
		return "C"
	case score >= 30:
		return "D"
	default:
		return "F"
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
