package analysis

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"sentinelbag/models"
)

const (
	circularWindow = int64(86400) // 24-hour detection window
	maxSamples     = 3            // max example patterns to include in output
)

type edge struct {
	from, to  string
	timestamp int64
}

// AnalyzeCircular detects A→B→A (2-hop) and A→B→C→A (3-hop) circular flows.
//
// Design notes:
//
//  1. Ratio-based scoring: penalty is derived from circular_patterns / total_txns.
//     Fixed count thresholds were fragile — 80 cycles in 1600 txns (5%) is normal
//     DeFi arbitrage noise; 80 cycles in 200 txns (40%) is a strong wash-trading signal.
//
//  2. Infrastructure exclusion: any cycle that routes through a known DEX program
//     or is performed by a clear market-maker (address appears on both sides of
//     token flows) is treated as legitimate rebalancing and not counted.
//
//  3. confidencePenalty scales the result down when sample size < 100.
//
// Penalty schedule (ratio = filtered_cycles / total_txns):
//
//	ratio > 25%  → 35 pts  (HIGH)
//	ratio > 15%  → 20 pts  (MEDIUM)
//	ratio >  5%  → 10 pts  (LOW)
//	ratio ≤  5%  →  0 pts  (CLEAN)
func AnalyzeCircular(txns []models.HeliusTransaction, tokenAddress string, infraShare float64, uniqueSenders int) models.AnalysisResult {
	edges := buildEdges(txns, tokenAddress)
	if len(edges) == 0 {
		return noDataResult("Circular Flow", "No token transfers found to analyze")
	}

	sort.Slice(edges, func(i, j int) bool {
		return edges[i].timestamp < edges[j].timestamp
	})

	bySender := map[string][]edge{}
	for _, e := range edges {
		bySender[e.from] = append(bySender[e.from], e)
	}

	// Build a set of market-maker addresses: wallets that appear on BOTH sides
	// of token flows with meaningful frequency are AMM pools / liquidity providers,
	// not wash traders. We compute this locally to avoid coupling to clustering.go.
	mmAddrs := buildMarketMakers(edges)

	seen := map[string]bool{}
	twoHop, threeHop := 0, 0
	skipped := 0
	var samples []models.CircularPattern

	for _, e1 := range edges {
		for _, e2 := range bySender[e1.to] {
			// 2-hop: A → B → A
			if e2.to == e1.from && abs64(e2.timestamp-e1.timestamp) <= circularWindow {
				// Exclude infra programs and market-maker wallets — these are
				// legitimate arbitrage / rebalance flows, not wash trading.
				if isLegitCycle(mmAddrs, e1.from, e1.to) {
					skipped++
					continue
				}
				key := dedupKey(e1.from, e1.to)
				if !seen[key] {
					seen[key] = true
					twoHop++
					if len(samples) < maxSamples {
						samples = append(samples, models.CircularPattern{
							Wallets:  []string{truncateAddr(e1.from), truncateAddr(e1.to)},
							HopCount: 2,
						})
					}
				}
				continue // don't check 3-hop from an already-matched 2-hop start
			}

			// 3-hop: A → B → C → A
			for _, e3 := range bySender[e2.to] {
				if e3.to == e1.from && abs64(e3.timestamp-e1.timestamp) <= circularWindow {
					if isLegitCycle(mmAddrs, e1.from, e1.to, e2.to) {
						skipped++
						continue
					}
					key := dedupKey(e1.from, e1.to, e2.to)
					if !seen[key] {
						seen[key] = true
						threeHop++
						if len(samples) < maxSamples {
							samples = append(samples, models.CircularPattern{
								Wallets:  []string{truncateAddr(e1.from), truncateAddr(e1.to), truncateAddr(e2.to)},
								HopCount: 3,
							})
						}
					}
				}
			}
		}
	}

	total := twoHop + threeHop
	ratio := 0.0
	if len(txns) > 0 {
		ratio = float64(total) / float64(len(txns))
	}
	penalty, severity, flag := scoreCircular(ratio)

	// Statistical confidence guard: small sample → reduce penalty
	penalty = confidencePenalty(penalty, len(txns))

	log.Printf("circular: %d patterns (%.1f%% ratio) — %d direct, %d triangular, %d infra/mm skipped (txns: %d)",
		total, ratio*100, twoHop, threeHop, skipped, len(txns))

	var detail string
	switch {
	case total == 0:
		detail = fmt.Sprintf(
			"No circular flow patterns detected in the 24h window (%d infrastructure cycles excluded)",
			skipped,
		)
	case severity == "CLEAN":
		// Ratio is within normal DeFi arbitrage noise — suppress as a warning
		detail = fmt.Sprintf(
			"Minimal circular flow — %.1f%% ratio (%d pattern(s)) is within normal DeFi arbitrage range; %d infra/MM cycles excluded",
			ratio*100, total, skipped,
		)
	default:
		detail = fmt.Sprintf(
			"%d circular pattern(s) detected (%.1f%% ratio): %d direct (A→B→A), %d triangular (A→B→C→A); %d infra/MM cycles excluded",
			total, ratio*100, twoHop, threeHop, skipped,
		)
	}
	if len(txns) < 100 {
		detail += fmt.Sprintf(" [low-confidence: %d txns sampled, penalty reduced]", len(txns))
	}

	return models.AnalysisResult{
		Rule:     "Circular Flow",
		Detail:   detail,
		Severity: severity,
		Flag:     flag,
		Score:    penalty,
		Metrics: models.EvidenceMetrics{
			TwoHopCount:   twoHop,
			ThreeHopCount: threeHop,
			TotalPatterns: total,
			Samples:       samples,
		},
	}
}

// buildMarketMakers returns addresses that appear as BOTH sender and receiver
// in the edge list with at least minMMEdges occurrences on each side.
// These are AMM pools / liquidity providers — their circular patterns are
// normal rebalancing, not wash trading.
func buildMarketMakers(edges []edge) map[string]bool {
	const minMMEdges = 3 // minimum appearances on each side to qualify
	fromCount := map[string]int{}
	toCount := map[string]int{}
	for _, e := range edges {
		fromCount[e.from]++
		toCount[e.to]++
	}
	mm := map[string]bool{}
	for addr, fc := range fromCount {
		if fc >= minMMEdges && toCount[addr] >= minMMEdges {
			mm[addr] = true
		}
	}
	return mm
}

// isLegitCycle returns true when any wallet in the cycle is a known DEX/protocol
// infrastructure address or a locally identified market-maker wallet.
// Such cycles represent arbitrage / rebalancing, not wash trading.
func isLegitCycle(mmAddrs map[string]bool, wallets ...string) bool {
	for _, w := range wallets {
		if IsKnownInfrastructure(w) || mmAddrs[w] {
			return true
		}
	}
	return false
}

func buildEdges(txns []models.HeliusTransaction, tokenAddress string) []edge {
	var edges []edge
	for _, tx := range txns {
		for _, tt := range tx.TokenTransfers {
			if tt.Mint != tokenAddress || tt.FromUserAccount == "" || tt.ToUserAccount == "" {
				continue
			}
			edges = append(edges, edge{
				from:      tt.FromUserAccount,
				to:        tt.ToUserAccount,
				timestamp: tx.Timestamp,
			})
		}
	}
	return edges
}

// scoreCircular derives a penalty from the ratio of circular patterns to total
// transactions. Ratio-based scoring is volume-agnostic: 80 cycles in 1600 txns
// (5%) signals normal DeFi arbitrage noise; 80 in 200 txns (40%) is a red flag.
//
//	ratio > 25% → 35 pts  HIGH
//	ratio > 15% → 20 pts  MEDIUM
//	ratio >  5% → 10 pts  LOW
//	ratio ≤  5% →  0 pts  CLEAN
func scoreCircular(ratio float64) (penalty int, severity, flag string) {
	switch {
	case ratio > 0.25:
		return 35, "HIGH", "CIRCULAR_FLOW"
	case ratio > 0.15:
		return 20, "MEDIUM", "CIRCULAR_FLOW"
	case ratio > 0.05:
		return 10, "LOW", "CIRCULAR_FLOW"
	default:
		return 0, "CLEAN", ""
	}
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func dedupKey(wallets ...string) string {
	sorted := make([]string, len(wallets))
	copy(sorted, wallets)
	sort.Strings(sorted)
	return strings.Join(sorted, "|")
}
