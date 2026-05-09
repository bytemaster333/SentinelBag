package analysis

import (
	"fmt"
	"sort"

	"sentinelbag/models"
)

// AnalyzeDiversity computes the Buyer Diversity Index (BDI) and identifies
// repeat recipient wallets that indicate bot-driven volume inflation.
//
// BDI = unique_recipient_wallets / total_transfer_events
//
//	BDI → 1.0 : every transfer reaches a new wallet (organic distribution)
//	BDI → 0.0 : same wallets recycled endlessly (scripted bot activity)
//
// Three scoring modes (mirrors clustering.go):
//
//  1. STRICT (small-cap / low DEX) — a genuine organic token should reach new
//     wallets at least 80% of the time. A BDI of 0.50 (which sounds like 50%
//     unique) is suspicious for a low-volume token.
//
//  2. ULTRA-LENIENT (totalTransfers > 1000) — JUP/USDC level.
//
//  3. LENIENT (totalTransfers > 300) — mid-volume DeFi.
//
//  4. STANDARD — low-volume tokens in non-strict context.
//
// Penalty schedule (strict):
//
//	BDI < 0.30  → 35 pts  (HIGH)
//	BDI < 0.60  → 20 pts  (MEDIUM)
//	BDI < 0.80  → 10 pts  (LOW)
//	BDI ≥ 0.80  →  0 pts  (CLEAN)
//
// Penalty schedule (ultra-lenient, totalTransfers > 1000):
//
//	BDI < 0.02  → 35 pts  (HIGH)
//	BDI < 0.06  → 20 pts  (MEDIUM)
//	BDI < 0.15  → 10 pts  (LOW)
//	BDI ≥ 0.15  →  0 pts  (CLEAN)
//
// Penalty schedule (lenient, totalTransfers > 300):
//
//	BDI < 0.05  → 35 pts  (HIGH)
//	BDI < 0.12  → 20 pts  (MEDIUM)
//	BDI < 0.25  → 10 pts  (LOW)
//	BDI ≥ 0.25  →  0 pts  (CLEAN)
//
// Penalty schedule (standard):
//
//	BDI < 0.10  → 35 pts  (HIGH)
//	BDI < 0.25  → 20 pts  (MEDIUM)
//	BDI < 0.50  → 10 pts  (LOW)
//	BDI ≥ 0.50  →  0 pts  (CLEAN)
func AnalyzeDiversity(txns []models.HeliusTransaction, tokenAddress string, precomputedInfraShare float64, uniqueSenders int) models.AnalysisResult {
	receiveCount := map[string]int{}
	totalTransfers := 0

	for _, tx := range txns {
		for _, tt := range tx.TokenTransfers {
			if tt.Mint != tokenAddress || tt.ToUserAccount == "" {
				continue
			}
			receiveCount[tt.ToUserAccount]++
			totalTransfers++
		}
	}

	if totalTransfers == 0 {
		return noDataResult("Buyer Diversity", "No token transfers found to compute diversity index")
	}

	uniqueWallets := len(receiveCount)
	bdi := float64(uniqueWallets) / float64(totalTransfers)

	tier := ClassifyTier(tokenAddress, uniqueSenders, precomputedInfraShare)
	strict := tier == TierStrict
	penalty, severity, flag := scoreDiversity(bdi, totalTransfers, strict)

	// Apply mode-appropriate confidence guard
	if strict {
		penalty = strictConfidencePenalty(penalty, len(txns))
	} else {
		penalty = confidencePenalty(penalty, len(txns))
	}

	// Top repeat buyers (most appearances as recipient)
	type walletCount struct {
		address string
		count   int
	}
	ranked := make([]walletCount, 0, uniqueWallets)
	for addr, cnt := range receiveCount {
		ranked = append(ranked, walletCount{addr, cnt})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].count > ranked[j].count
	})

	topN := min(3, len(ranked))
	repeatBuyers := make([]models.WalletShare, topN)
	for i := 0; i < topN; i++ {
		repeatBuyers[i] = models.WalletShare{
			Address: truncateAddr(ranked[i].address),
			Share:   float64(ranked[i].count) / float64(totalTransfers),
			Volume:  float64(ranked[i].count),
		}
	}

	var modePrefix string
	switch tier {
	case TierStrict:
		modePrefix = "[strict] "
	case TierBluechip:
		modePrefix = "[bluechip] "
	}

	var detail string
	if severity == "CLEAN" {
		detail = fmt.Sprintf(
			"%sHealthy diversity — index: %.2f (%d unique recipients / %d transfers)",
			modePrefix, bdi, uniqueWallets, totalTransfers,
		)
	} else {
		detail = fmt.Sprintf(
			"%sLow diversity index: %.2f — %d unique recipients across %d transfers",
			modePrefix, bdi, uniqueWallets, totalTransfers,
		)
	}
	if len(txns) < 100 {
		detail += fmt.Sprintf(" [low-confidence: %d txns]", len(txns))
	}

	return models.AnalysisResult{
		Rule:     "Buyer Diversity",
		Detail:   detail,
		Severity: severity,
		Flag:     flag,
		Score:    penalty,
		Metrics: models.EvidenceMetrics{
			UniqueWallets:  uniqueWallets,
			TotalTransfers: totalTransfers,
			DiversityIndex: bdi,
			RepeatBuyers:   repeatBuyers,
		},
	}
}

// scoreDiversity selects thresholds based on mode.
func scoreDiversity(bdi float64, totalTransfers int, strict bool) (penalty int, severity, flag string) {
	if strict {
		// Small-cap organic token: genuine distribution should reach ≥80% new wallets.
		// A meme token with BDI=0.50 has 50% repeat recipients — suspicious at low volume.
		switch {
		case bdi < 0.30:
			return 35, "HIGH", "BOT_ACTIVITY"
		case bdi < 0.60:
			return 20, "MEDIUM", "BOT_ACTIVITY"
		case bdi < 0.80:
			return 10, "LOW", ""
		default:
			return 0, "CLEAN", ""
		}
	}

	switch {
	case totalTransfers > 1000:
		// Ultra-lenient: JUP/USDC level — institutional recipients repeat constantly.
		switch {
		case bdi < 0.02:
			return 35, "HIGH", "BOT_ACTIVITY"
		case bdi < 0.06:
			return 20, "MEDIUM", "BOT_ACTIVITY"
		case bdi < 0.15:
			return 10, "LOW", ""
		default:
			return 0, "CLEAN", ""
		}

	case totalTransfers > 300:
		// Lenient: mid-volume DeFi tokens tolerate more repetition than small caps.
		switch {
		case bdi < 0.05:
			return 35, "HIGH", "BOT_ACTIVITY"
		case bdi < 0.12:
			return 20, "MEDIUM", "BOT_ACTIVITY"
		case bdi < 0.25:
			return 10, "LOW", ""
		default:
			return 0, "CLEAN", ""
		}

	default:
		// Standard thresholds for low-volume / non-strict tokens.
		switch {
		case bdi < 0.10:
			return 35, "HIGH", "BOT_ACTIVITY"
		case bdi < 0.25:
			return 20, "MEDIUM", "BOT_ACTIVITY"
		case bdi < 0.50:
			return 10, "LOW", ""
		default:
			return 0, "CLEAN", ""
		}
	}
}
