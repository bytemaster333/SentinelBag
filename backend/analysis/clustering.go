package analysis

import (
	"fmt"
	"log"
	"sort"

	"sentinelbag/models"
)

// AnalyzeClustering measures volume concentration using the Herfindahl–Hirschman Index (HHI).
//
// Three scoring modes depending on token character:
//
//  1. STRICT (small-cap / low DEX) — totalTxns < 300 AND infraShare < 0.10
//     Any wallet holding >30% OR HHI > 0.15 triggers maximum penalty (-40).
//     The confidence guard is limited to 10–20% max reduction.
//     Goal: 150-txn Pump.fun token with 45% concentration → D or F grade.
//
//  2. LENIENT (high-volume / infra-heavy) — totalTxns > 300 OR infraShare > 0.40
//     Applies to USDC, JUP, and other tokens with significant institutional flow.
//     Higher HHI is expected due to exchange and LP program involvement.
//
//  3. STANDARD — everything else (mid-cap, moderate DEX use)
//
// Penalty schedule (strict):
//
//	HHI > 0.15 OR top1 > 30%  → 40 pts  (HIGH)
//	HHI > 0.10                → 25 pts  (HIGH)
//	HHI > 0.05                → 10 pts  (MEDIUM)
//	otherwise                 →  0 pts  (CLEAN)
//
// Penalty schedule (standard):
//
//	HHI > 0.40  → 40 pts  (HIGH)
//	HHI > 0.20  → 25 pts  (HIGH)
//	HHI > 0.10  → 10 pts  (MEDIUM)
//	otherwise   →  0 pts  (CLEAN)
//
// Penalty schedule (lenient):
//
//	HHI > 0.50  → 40 pts  (HIGH)
//	HHI > 0.30  → 25 pts  (HIGH)
//	HHI > 0.15  → 10 pts  (MEDIUM)
//	otherwise   →  0 pts  (CLEAN)
func AnalyzeClustering(txns []models.HeliusTransaction, tokenAddress string, precomputedInfraShare float64, uniqueSenders int) models.AnalysisResult {
	senderVolume := map[string]float64{}
	receiverVolume := map[string]float64{}
	totalVolume := 0.0

	for _, tx := range txns {
		for _, tt := range tx.TokenTransfers {
			if tt.Mint != tokenAddress {
				continue
			}
			if tt.FromUserAccount != "" {
				senderVolume[tt.FromUserAccount] += tt.TokenAmount
				totalVolume += tt.TokenAmount
			}
			if tt.ToUserAccount != "" {
				receiverVolume[tt.ToUserAccount] += tt.TokenAmount
			}
		}
	}

	if totalVolume == 0 {
		return noDataResult("Wallet Clustering", "No token transfer volume found in sampled transactions")
	}

	// Early exit: if infrastructure wallets (AMMs, aggregators, mint authorities) account
	// for more than half of total volume AND the highest-volume non-infrastructure sender
	// is below 40%, this token's concentration pattern is driven by protocol activity,
	// not wash trading. Return CLEAN without running the full HHI calculation.
	{
		infraVol := 0.0
		maxNonInfraVol := 0.0
		infraCount := 0
		for addr, vol := range senderVolume {
			if _, ok := IsInfrastructureWallet(addr); ok {
				infraVol += vol
				infraCount++
			} else if vol > maxNonInfraVol {
				maxNonInfraVol = vol
			}
		}
		infraFrac := infraVol / totalVolume
		nonInfraTopShare := maxNonInfraVol / totalVolume
		if nonInfraTopShare < 0.40 && infraFrac > 0.50 {
			log.Printf("clustering: infra-dominated (%.0f%% infra, %.0f%% non-infra top, %d wallets) — CLEAN",
				infraFrac*100, nonInfraTopShare*100, infraCount)
			return models.AnalysisResult{
				Rule: "Wallet Clustering",
				Detail: fmt.Sprintf(
					"Volume dominated by infrastructure (Raydium, Jupiter, etc.) — no concentration penalty applied (%d infra wallets excluded)",
					infraCount,
				),
				Severity: "CLEAN",
				Flag:     "",
				Score:    0,
			}
		}
	}

	// Classify scoring tier using the centrally-computed infraShare from the handler.
	// BIDIRECTIONAL market-maker discount is disabled in strict mode — wash traders
	// that happen to both buy and sell must not benefit from the 50% HHI reduction.
	totalTxns := len(txns)
	tier := ClassifyTier(tokenAddress, uniqueSenders, precomputedInfraShare)
	strict := tier == TierStrict

	// Pass 2: compute adjusted HHI with mode-aware categorisation.
	//   infra                          → ×0.20  (80% reduction)
	//   market maker (non-strict only) → ×0.50  (50% reduction)
	//   wash / normal                  → ×1.00
	hhi := 0.0
	infraShare := 0.0
	infraHits := 0
	for addr, vol := range senderVolume {
		share := vol / totalVolume
		var category string
		switch {
		case IsKnownInfrastructure(addr):
			hhi += (share * share) * 0.20
			infraShare += share
			infraHits++
			category = "INFRA"
		case !strict && isBidirectional(addr, senderVolume, receiverVolume, totalVolume):
			hhi += (share * share) * 0.50
			category = "BIDIRECTIONAL"
		default:
			hhi += share * share
			if strict && isBidirectional(addr, senderVolume, receiverVolume, totalVolume) {
				category = "WASH"
			} else {
				category = "NORMAL"
			}
		}
		if share > 0.02 {
			log.Printf("clustering: sender share=%.1f%% category=%-13s addr=%s",
				share*100, category, addr)
		}
	}

	log.Printf("clustering: %d/%d senders are known infra (infraShare: %.0f%%, hhi: %.3f, txns: %d)",
		infraHits, len(senderVolume), infraShare*100, hhi, len(txns))

	// Sort senders for top-N display; extract top1Share for strict mode check
	type walletVol struct {
		address string
		volume  float64
	}
	ranked := make([]walletVol, 0, len(senderVolume))
	for addr, vol := range senderVolume {
		ranked = append(ranked, walletVol{addr, vol})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].volume > ranked[j].volume
	})

	top1Share := 0.0
	if len(ranked) > 0 {
		top1Share = ranked[0].volume / totalVolume
	}

	topN := min(3, len(ranked))
	top3Share := 0.0
	topWallets := make([]models.WalletShare, topN)
	for i := 0; i < topN; i++ {
		share := ranked[i].volume / totalVolume
		top3Share += share
		topWallets[i] = models.WalletShare{
			Address: truncateAddr(ranked[i].address),
			Share:   share,
			Volume:  ranked[i].volume,
		}
	}

	penalty, severity, flag := scoreHHI(hhi, top1Share, totalTxns, infraShare, strict)

	// TierBluechip tokens organically attract concentration from exchanges and LPs;
	// halve the raw HHI penalty to avoid penalising legitimate institutional flow.
	if tier == TierBluechip {
		penalty = penalty / 2
	}

	// Apply mode-appropriate confidence guard
	if strict {
		penalty = strictConfidencePenalty(penalty, totalTxns)
	} else {
		penalty = confidencePenalty(penalty, totalTxns)
	}

	log.Printf("clustering: tier=%s penalty=%d severity=%s top1=%.0f%% hhi=%.3f infraShare=%.1f%%",
		tierName(tier), penalty, severity, top1Share*100, hhi, infraShare*100)

	var detail string
	var modePrefix string
	switch tier {
	case TierStrict:
		modePrefix = "[strict] "
	case TierBluechip:
		modePrefix = "[bluechip] "
	}
	infraSuffix := ""
	if infraHits > 0 {
		infraSuffix = fmt.Sprintf(", %d infra excluded", infraHits)
	}
	if severity == "CLEAN" {
		detail = fmt.Sprintf(
			"%sHealthy distribution — adjusted HHI: %.3f, top wallet: %.0f%%, top 3: %.0f%% (%d senders%s)",
			modePrefix, hhi, top1Share*100, top3Share*100, len(ranked), infraSuffix,
		)
	} else {
		detail = fmt.Sprintf(
			"%sAdjusted HHI: %.3f — top wallet: %.0f%%, top 3: %.0f%% of volume (%d senders, infra: %.0f%%%s)",
			modePrefix, hhi, top1Share*100, top3Share*100, len(ranked), infraShare*100, infraSuffix,
		)
	}
	if totalTxns < 100 {
		detail += fmt.Sprintf(" [low-confidence: %d txns]", totalTxns)
	}

	return models.AnalysisResult{
		Rule:     "Wallet Clustering",
		Detail:   detail,
		Severity: severity,
		Flag:     flag,
		Score:    penalty,
		Metrics: models.EvidenceMetrics{
			TopWallets:   topWallets,
			HHI:          hhi,
			Top3Share:    top3Share,
			TotalSenders: len(ranked),
		},
	}
}

// scoreHHI returns a penalty based on the adjusted HHI value and the scoring mode.
//
// strict=true: tight thresholds; any wallet >30% OR HHI >0.15 triggers max penalty.
// lenient:     loose thresholds for high-volume / infra-heavy tokens.
// standard:    baseline for mid-cap tokens.
func scoreHHI(hhi, top1Share float64, totalTxns int, infraShare float64, strict bool) (penalty int, severity, flag string) {
	if strict {
		// Strict mode is reserved for tokens with <400 unique senders OR
		// insufficient ecosystem evidence. Concentration here is unambiguous
		// manipulation, so penalties are aggressive enough to land Grade F.
		//
		//   top1Share > 50% → 80 pts (auto-F: single wallet owns more than half)
		//   top1Share > 25% → 50 pts (auto-D: dominant single holder)
		//   ...less concentrated cases keep the calibrated mid-tier penalties.
		if top1Share > 0.50 {
			return 80, "HIGH", "HIGH_CONCENTRATION"
		}
		if top1Share > 0.25 || hhi > 0.15 {
			return 50, "HIGH", "HIGH_CONCENTRATION"
		}
		switch {
		case top1Share > 0.15 || hhi > 0.10:
			return 25, "HIGH", "HIGH_CONCENTRATION"
		case hhi > 0.05:
			return 10, "MEDIUM", ""
		default:
			return 0, "CLEAN", ""
		}
	}

	lenient := totalTxns > 300 || infraShare > 0.40
	if lenient {
		switch {
		case hhi > 0.50:
			return 40, "HIGH", "HIGH_CONCENTRATION"
		case hhi > 0.30:
			return 25, "HIGH", "HIGH_CONCENTRATION"
		case hhi > 0.15:
			return 10, "MEDIUM", ""
		default:
			return 0, "CLEAN", ""
		}
	}

	// Standard thresholds
	switch {
	case hhi > 0.40:
		return 40, "HIGH", "HIGH_CONCENTRATION"
	case hhi > 0.20:
		return 25, "HIGH", "HIGH_CONCENTRATION"
	case hhi > 0.10:
		return 10, "MEDIUM", ""
	default:
		return 0, "CLEAN", ""
	}
}

func tierName(tier ScoringTier) string {
	switch tier {
	case TierStrict:
		return "STRICT"
	case TierBluechip:
		return "BLUECHIP"
	default:
		return "STANDARD"
	}
}
