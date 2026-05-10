package analysis

import (
	"log"
	"strings"

	"sentinelbag/models"
)

// noDataResult returns a clean, penalty-free result when a heuristic has no data to work with.
func noDataResult(rule, detail string) models.AnalysisResult {
	return models.AnalysisResult{
		Rule:     rule,
		Detail:   detail,
		Severity: "CLEAN",
		Flag:     "",
		Score:    0,
	}
}

// truncateAddr shortens a base58 address to first4…last4 for display.
func truncateAddr(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:4] + "…" + addr[len(addr)-4:]
}

// ScoringTier captures the Proof of Ecosystem classification for a token.
// All three heuristics use the same tier so scoring is consistent.
type ScoringTier int

const (
	TierStrict   ScoringTier = iota // infraShare < 5% — zero tolerance regardless of volume
	TierStandard                     // infraShare ≥ 5%, mid-volume — balanced penalties
	TierBluechip                     // uniqueSenders > 400 + infraShare ≥ 5% — most lenient
)

// ClassifyTier implements the Proof of Ecosystem gate.
//
// HARD GATE — uniqueSenders < 30 → TierStrict. No exceptions.
// In a 1000-transaction sample, even the largest blue-chip tokens rarely
// surface 400+ distinct senders because volume is concentrated through CEX
// hot wallets and AMM pools. 30 is the minimum-signal floor: below this,
// the sample is too sparse for any statistical inference to be meaningful.
//
// Above the floor:
//   - infraShare < 1%             → TierStrict (no DEX evidence)
//   - pump suffix + infra < 15%   → TierStrict (bonding-curve self-traffic)
//   - !pump + uniqueSenders > 30  → TierBluechip
//   - otherwise                   → TierStandard
func ClassifyTier(tokenAddress string, uniqueSenders int, infraShare float64) ScoringTier {
	// SEAL: under 30 unique senders, no token escapes strict mode — period.
	if uniqueSenders < 30 {
		return TierStrict
	}

	isPump := strings.HasSuffix(strings.ToLower(tokenAddress), "pump")

	// Proof of Ecosystem: insufficient infra interaction → always strict
	if infraShare < 0.01 {
		return TierStrict
	}
	// Pump tokens need a tighter bar (15%) to exit strict mode;
	// bonding-curve activity easily reaches 5% without real ecosystem integration
	if isPump && infraShare < 0.15 {
		return TierStrict
	}
	// Above floor + ecosystem-connected + non-pump → bluechip
	if !isPump {
		return TierBluechip
	}
	return TierStandard
}

// TierLabel returns a human-readable name for a ScoringTier (used in log lines).
func TierLabel(tier ScoringTier) string {
	switch tier {
	case TierStrict:
		return "STRICT"
	case TierBluechip:
		return "BLUECHIP"
	default:
		return "STANDARD"
	}
}

// excludeFromInfraShare lists addresses that must NOT be counted as "infra"
// when computing infraShare. These programs appear in virtually every SPL token
// transfer (making infraShare ≈ 100% for all tokens if included), or they represent
// the token's own internal mechanics (Pump.fun bonding curve) rather than external
// ecosystem integration.
var excludeFromInfraShare = map[string]bool{
	"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA":  true, // SPL Token Program — in every transfer
	"ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJe1brs": true, // ATA Program — in every transfer
	"So11111111111111111111111111111111111111112":    true, // Wrapped SOL mint — ubiquitous
	"11111111111111111111111111111111":               true, // System Program — in every tx
	"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P":  true, // Pump.fun bonding curve — own mechanics
}

// isDEXInfrastructure reports whether addr is a known DEX/protocol address AND
// should be counted towards infraShare. Excludes ubiquitous SPL programs and the
// Pump.fun bonding curve which would otherwise inflate infraShare for all tokens.
func isDEXInfrastructure(addr string) bool {
	return knownInfrastructure[addr] && !excludeFromInfraShare[addr]
}

// ecosystemSources lists Helius source field values that represent genuine external
// DeFi protocol involvement. PUMP_FUN is intentionally excluded — it is the token's
// own bonding curve, not an external ecosystem interaction.
var ecosystemSources = map[string]bool{
	"RAYDIUM":  true,
	"ORCA":     true,
	"JUPITER":  true,
	"METEORA":  true,
	"LIFINITY": true,
	"OPENBOOK": true,
	"SERUM":    true,
	"DRIFT":    true,
	"MANGO":    true,
	"MARINADE": true,
	"PHOENIX":  true,
	"ZETA":     true,
}

// ComputeInfraShare returns the fraction of token-relevant transactions that
// demonstrably involve known DeFi infrastructure. This is the Proof of Ecosystem
// signal used by ClassifyTier — and must be computed once in the handler so that
// all three heuristics share the same authoritative value.
//
// Uses tx.Source ∈ ecosystemSources (e.g. "RAYDIUM", "JUPITER") exclusively.
// Helius identifies the originating protocol regardless of pool token accounts,
// so this is far more reliable than checking individual wallet addresses.
//
// The address-based fallback has been removed: if Helius returns source="UNKNOWN"
// for all transactions, we return 0 and let ClassifyTier assign TierStrict.
// Giving benefit-of-the-doubt on ambiguous data has proven dangerous in practice.
func ComputeInfraShare(txns []models.HeliusTransaction, tokenMint string) float64 {
	total := 0
	sourceBased := 0
	// Track which source labels appear so we can audit what's counted vs ignored.
	sourceBreakdown := map[string]int{}

	for _, tx := range txns {
		relevant := false
		for _, tt := range tx.TokenTransfers {
			if tt.Mint == tokenMint {
				relevant = true
				break
			}
		}
		if !relevant {
			continue
		}
		total++
		sourceBreakdown[tx.Source]++
		if ecosystemSources[tx.Source] {
			sourceBased++
		}
	}

	if total == 0 {
		return 0
	}

	// Diagnostic: log every source label seen so it's obvious if PUMP_FUN or
	// SYSTEM_PROGRAM are slipping through, and which ecosystem hits drove the share.
	share := float64(sourceBased) / float64(total)
	log.Printf("infraShare: %.1f%% (%d/%d ecosystem hits) — sources: %v",
		share*100, sourceBased, total, sourceBreakdown)
	return share
}

// ComputeUniqueSenders returns the count of distinct wallet addresses that sent
// the target token in the provided transaction set.
//
// This is used instead of len(txns) to gate TierBluechip: a wash trader can
// trivially inflate transaction count with a small wallet cluster, but creating
// hundreds of independently funded wallets is prohibitively expensive.
func ComputeUniqueSenders(txns []models.HeliusTransaction, tokenMint string) int {
	senders := map[string]bool{}
	for _, tx := range txns {
		for _, tt := range tx.TokenTransfers {
			if tt.Mint == tokenMint && tt.FromUserAccount != "" {
				senders[tt.FromUserAccount] = true
			}
		}
	}
	return len(senders)
}

// confidencePenalty scales a raw penalty down based on sample size.
// Used for tokens in normal or lenient mode.
// Thresholds are aligned with MinSampleSize=150 (full confidence at 150+ txns).
//
//	sampleSize < 50  → 30% of penalty (70% reduction — very low confidence)
//	sampleSize < 150 → 60% of penalty (40% reduction — below gate threshold)
//	otherwise        → full penalty
func confidencePenalty(penalty, sampleSize int) int {
	switch {
	case sampleSize < 50:
		return (penalty * 3) / 10
	case sampleSize < 150:
		return (penalty * 6) / 10
	default:
		return penalty
	}
}

// strictConfidencePenalty applies a much lighter reduction than confidencePenalty.
// Used when isStrictMode is true — a clear wash-trading signal from 50 transactions
// should not benefit from the same leniency as an uncertain 50-txn sample of USDC.
//
//	sampleSize < 50  → 80% of penalty (20% reduction max)
//	sampleSize < 150 → 90% of penalty (10% reduction max)
//	otherwise        → full penalty
func strictConfidencePenalty(penalty, sampleSize int) int {
	switch {
	case sampleSize < 50:
		return (penalty * 8) / 10
	case sampleSize < 150:
		return (penalty * 9) / 10
	default:
		return penalty
	}
}

// knownInfrastructure contains well-known DEX programs, protocol addresses,
// bridges, and lending markets that participate in legitimate high-volume activity.
// Their HHI contribution is reduced by 80% (×0.20) to prevent false positives
// on stablecoins, major DeFi tokens, and bridged assets.
var knownInfrastructure = map[string]bool{
	// ── Raydium ─────────────────────────────────────────────────────────────
	"675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8": true, // AMM v4
	"CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK": true, // CLMM
	"5quBtoiQqxF9Jv6KYKctB59NT3gtFD2SqTKHTAE98KP":  true, // AMM v3 (legacy)
	"HWy1jotHpo6UqeQxx49dpYYdQB8wj9Qk9MdxwjLvDHB8": true, // Raydium Routing

	// ── Orca ────────────────────────────────────────────────────────────────
	"whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc":  true, // Whirlpools
	"9W959DqEETiGZocYWCQPaJ6sBmUzgfxXfqGeTEdp3aQP": true, // Swap v1 (legacy)

	// ── Jupiter ─────────────────────────────────────────────────────────────
	"JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4": true, // Aggregator v6
	"JUP4Fb2cqiRUcaTHdrPC8h2gNsA2ETXiPDD33WcGuJB":  true, // Aggregator v4
	"JUP3c2Uh3WA4Ng34tw6kPd2G4YFD3zyGnRqiGo7t24":  true, // Aggregator v3

	// ── Pump.fun ────────────────────────────────────────────────────────────
	"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P": true, // Bonding curve program

	// ── Meteora ─────────────────────────────────────────────────────────────
	"LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo": true, // DLMM
	"Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EkEV2Vp": true, // AMM pools

	// ── Saber / Mercurial ────────────────────────────────────────────────────
	"SSwpkEEcbUqx4vtoEByFjSkhKdCT862DNVb52nZg1UZ": true, // Saber StableSwap
	"MERLuDFBMmsHnsBPZw2sDQZHvXFMwp8EdjudcU2pgJe": true, // Mercurial Finance

	// ── Serum / OpenBook ─────────────────────────────────────────────────────
	"9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin": true, // Serum DEX v3
	"srmqPvymJeFKQ4zGQed1GFppgkRHL9kaELCbyksJtPX":  true, // Serum open orders
	"opnb2LAfJYbRMAHHvqjCwQxanZn7n734aCpyqpaKnbJ":  true, // OpenBook v2

	// ── Wormhole / Portal ────────────────────────────────────────────────────
	"worm2ZoG2kUd4vFXhvjh93UUH596ayRfgQ2MgjNMTth":  true, // Wormhole core bridge
	"DZnkkTmCiFWfYTfT41X3Rd1kDgozqzxWaHqsyD712YBi": true, // Wormhole token bridge
	"3u8hJUVTA4jH1wYAyUur7FFZVQ8H635K3tSHHF4ssjQ5": true, // Wormhole NFT bridge

	// ── Lending protocols ────────────────────────────────────────────────────
	"So1endDq2YkqhipRh3WViPa8hdiSpxWy6z3Z6tMCpAo":  true, // Solend
	"MFv2hWf31Z9kbCa1snEPdcgp168vLs2YNsNXC7Y9qHj":  true, // marginfi v1
	"KLend2g3cP87fffoy8q1mQqGKjrxjC8boSyAYavgmjD":  true, // Kamino Lending
	"Port7uDYB3wkM4GE6HDANXpo9wGfM59BpA4JYiMoq63": true,  // Port Finance

	// ── Drift / Mango ────────────────────────────────────────────────────────
	"dRiftyHA39MWEi3m9aunc5MzRF1JYuBsbn6VPcn33UH":  true, // Drift Protocol v2
	"4MangoMjqJ2firMokCjjGgoK8d4MXcrgL7XJaL3w6fVg": true, // Mango v4

	// ── Jupiter ancillary programs ───────────────────────────────────────────
	"jupoNjAxXgZ4rjzxzPMP4XXi1yrBVGJQqRsJBbFGZMh": true, // Jupiter Lock
	"DCA265Vj8a9CEuX1eb1LWRnDT7uK72pFxkSRkJ18Jfkm": true, // Jupiter DCA
	"j1o2qRpjcyUwEvwtcfhEQefh773ZgjxcVRry7LDqg5X":  true, // Jupiter Limit Orders

	// ── Phoenix DEX ──────────────────────────────────────────────────────────
	"PhoeNiXZ8ByJGLkxNfZRnkUfjvmuYqLR89jjFHGqdXY": true, // Phoenix v1

	// ── Zeta / Drift perps ────────────────────────────────────────────────────
	"ZETAxsqBRek56DhiGXrn75yj2NHU3aYUnxvHXpkf3aD": true, // Zeta Markets

	// ── Stake / liquid stake ──────────────────────────────────────────────────
	"MarBmsSgKXdrN1egZf5sqe1TMai9K1rChYNDJgjq7aD":  true, // Marinade Finance
	"5oVNBeEEQvYi1cX3ir8Dx5n1P7pdxydbGF2X4TxVusJm": true, // Lido (stSOL)
	"mSoLzYCxHdYgdzU16g5QSh3i5K3z3KZK7ytfqcJm7So":  true, // Marinade mSOL mint

	// ── CEX / Custodian hot wallets ───────────────────────────────────────────
	// Large exchange deposit and custody wallets naturally dominate transfer volume
	// for blue-chip tokens. Their HHI contribution is reduced by 80% (×0.20).
	//
	// To find missing addresses: look for "clustering: sender share=XX% category=NORMAL"
	// lines in the server log, paste the full address into Solscan, and add below.
	//
	// ── From live analysis (USDC / BONK / JUP) ───────────────────────────────
	// Replace each placeholder with the full address from the server log line:
	//   "clustering: sender share=XX.X% category=NORMAL addr=<FULL_ADDRESS>"
	//
	// "FULL_ADDRESS_6EL9xxV2Dv": true, // USDC whale/bridge   (6EL9…V2Dv in UI)
	// "FULL_ADDRESS_3ADzxxEFib": true, // BONK exchange wallet (3ADz…EFib in UI)
	// "FULL_ADDRESS_CAi4xxQijo": true, // JUP exchange wallet  (CAi4…Qijo in UI)
	//
	// ── Known Solana CEX addresses (publicly documented) ─────────────────────
	"AC5RDfQFmDS1deWZos921JfqscXdByf8BKHs5ACWjtW2":  true, // Binance hot wallet 1
	"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM": true, // Binance hot wallet 2
	"H8sMJSCQxfKiFTCfDR3DUMLPwcRbM61LGFJ8N4dK3WjS": true, // Binance hot wallet 3
	"2ojv9BAiHUrvsm9gxDe7fJSzbNZSJcxZvf8dqmWGHG8S": true, // Coinbase custody
	"CakcnaRDHka2gXyfxNhasEjWwXdqfZNiLHPmx7YJUJKN": true, // Coinbase Prime
	"GJRs4FwHtemZ5ZE9x3FNvJ8TMwitKTh21yxdRPqn7npE": true, // Kraken hot wallet
	"FWznbcNXWQuHTawe9RxvQ2LdCENssh12dsznf4RiouN5": true, // OKX hot wallet
	"5VCwKtCXgCJ6kit5FybXjvriW3xELsFDhx5Lt2Xokgw2": true, // Bybit deposit wallet
	"A77HErqtfN1hLLpvZ9pGtu7my2FdkYh1RCt3Aa6n13kQ": true, // Bybit hot wallet

	// ── Addresses identified from live analysis logs ──────────────────────────
	// To add new entries: run the backend, observe "clustering: sender … category=NORMAL"
	// log lines for addresses with high share, look them up on Solscan, and paste here.

	// ── SPL / system programs ────────────────────────────────────────────────
	"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA":  true, // SPL Token Program
	"ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJe1brs": true, // Associated Token Program
	"So11111111111111111111111111111111111111112":    true, // Wrapped SOL mint
	"11111111111111111111111111111111":               true, // System Program
}

// IsKnownInfrastructure reports whether addr is a known DEX program or protocol address.
func IsKnownInfrastructure(addr string) bool {
	return knownInfrastructure[addr]
}

// isBidirectional reports whether a wallet significantly appears on BOTH sides of token
// transfers (>5% of volume as sender AND >5% as receiver). Such wallets are almost
// always DEX liquidity pools or market makers, not wash traders.
func isBidirectional(addr string, senderVol, receiverVol map[string]float64, total float64) bool {
	if total == 0 {
		return false
	}
	return senderVol[addr]/total > 0.05 && receiverVol[addr]/total > 0.05
}
