package analysis

import (
	"testing"

	"sentinelbag/models"
)

// clusterMint is a non-pump token used in concentration tests.
const clusterMint = "CLUSTERTESTmint1111111111111111111111111111"

// clusterPumpMint ends in "pump" — ClassifyTier treats it as a pump token.
const clusterPumpMint = "CLUSTERTESTPUMPtokenaddresspump"

func TestClustering_CleanDistribution(t *testing.T) {
	// 10 transfers from 10 distinct senders, equal amounts.
	// With uniqueSenders=500 and infraShare=0.10 → TierBluechip.
	// HHI = 10 × 0.01 = 0.10, which is not strictly > 0.10 → CLEAN.
	senders := []string{
		"senderA", "senderB", "senderC", "senderD", "senderE",
		"senderF", "senderG", "senderH", "senderI", "senderJ",
	}
	var txns []models.HeliusTransaction
	for i, s := range senders {
		txns = append(txns, makeTx(clusterMint, s, "recipient", 100, int64(i)))
	}
	result := AnalyzeClustering(txns, clusterMint, 0.10, 500)
	if result.Severity != "CLEAN" {
		t.Errorf("expected CLEAN, got %s (score %d): %s", result.Severity, result.Score, result.Detail)
	}
	if result.Score != 0 {
		t.Errorf("expected score 0, got %d", result.Score)
	}
}

func TestClustering_HighConcentration(t *testing.T) {
	// 150 txns: 1 sender controls ~99% of volume, 10 others share the rest.
	// clusterPumpMint + uniqueSenders=500 + infraShare=0.20 → TierStandard (no halving).
	// HHI ≈ 0.99 > 0.40 → 40 pts; 150 txns → full confidence penalty → score 40.
	var txns []models.HeliusTransaction
	for i := 0; i < 140; i++ {
		txns = append(txns, makeTx(clusterPumpMint, "dominant", "recv", 1.0, int64(i)))
	}
	others := []string{"w1", "w2", "w3", "w4", "w5", "w6", "w7", "w8", "w9", "w10"}
	for i, s := range others {
		txns = append(txns, makeTx(clusterPumpMint, s, "recv", 0.07, int64(140+i)))
	}
	result := AnalyzeClustering(txns, clusterPumpMint, 0.20, 500)
	if result.Severity != "HIGH" {
		t.Errorf("expected HIGH, got %s (score %d): %s", result.Severity, result.Score, result.Detail)
	}
	if result.Score != 40 {
		t.Errorf("expected score 40, got %d", result.Score)
	}
}

func TestClustering_InfrastructureExcluded(t *testing.T) {
	// Raydium AMM v4 sends 90% of volume. Non-infra top share ≈ 1%.
	// Early-exit logic should detect infra dominance and return CLEAN.
	const raydiumV4 = "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8"
	var txns []models.HeliusTransaction
	for i := 0; i < 9; i++ {
		txns = append(txns, makeTx(clusterMint, raydiumV4, "recv", 100, int64(i)))
	}
	txns = append(txns, makeTx(clusterMint, "normalWallet", "recv", 10, 9))
	result := AnalyzeClustering(txns, clusterMint, 0.10, 500)
	if result.Severity != "CLEAN" {
		t.Errorf("expected CLEAN (infra dominated), got %s (score %d): %s",
			result.Severity, result.Score, result.Detail)
	}
	if result.Score != 0 {
		t.Errorf("expected score 0, got %d", result.Score)
	}
}

func TestClustering_NoVolume(t *testing.T) {
	result := AnalyzeClustering(nil, clusterMint, 0, 0)
	if result.Severity != "CLEAN" {
		t.Errorf("expected CLEAN for empty txns, got %s", result.Severity)
	}
	if result.Score != 0 {
		t.Errorf("expected score 0, got %d", result.Score)
	}
}
