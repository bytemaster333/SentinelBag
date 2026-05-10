package analysis

import (
	"testing"

	"sentinelbag/models"
)

const divMint = "DIVERSITYTESTmint1111111111111111111111111"

func TestDiversity_HighBDI(t *testing.T) {
	// 10 transfers each reaching a distinct recipient — BDI = 1.0.
	// TierBluechip (uniqueSenders=500, infraShare=0.10): standard thresholds, BDI ≥ 0.50 → CLEAN.
	var txns []models.HeliusTransaction
	recipients := []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9", "r10"}
	for i, r := range recipients {
		txns = append(txns, makeTx(divMint, "sender", r, 1.0, int64(i)))
	}
	result := AnalyzeDiversity(txns, divMint, 0.10, 500)
	if result.Severity != "CLEAN" {
		t.Errorf("expected CLEAN for BDI=1.0, got %s (score %d): %s",
			result.Severity, result.Score, result.Detail)
	}
	if result.Score != 0 {
		t.Errorf("expected score 0, got %d", result.Score)
	}
	if result.Metrics.DiversityIndex != 1.0 {
		t.Errorf("expected BDI 1.0, got %.2f", result.Metrics.DiversityIndex)
	}
}

func TestDiversity_LowBDI(t *testing.T) {
	// 20 transfers to only 2 recipients — BDI = 2/20 = 0.10.
	// TierStrict (uniqueSenders=10): BDI < 0.30 → HIGH.
	var txns []models.HeliusTransaction
	for i := 0; i < 10; i++ {
		txns = append(txns, makeTx(divMint, "sender", "recipientA", 1.0, int64(i)))
		txns = append(txns, makeTx(divMint, "sender", "recipientB", 1.0, int64(10+i)))
	}
	result := AnalyzeDiversity(txns, divMint, 0, 10)
	if result.Severity != "HIGH" {
		t.Errorf("expected HIGH for BDI=0.10, got %s (score %d): %s",
			result.Severity, result.Score, result.Detail)
	}
	if result.Score == 0 {
		t.Errorf("expected non-zero penalty, got 0")
	}
	if result.Metrics.DiversityIndex != 0.10 {
		t.Errorf("expected BDI 0.10, got %.2f", result.Metrics.DiversityIndex)
	}
}

func TestDiversity_Empty(t *testing.T) {
	result := AnalyzeDiversity(nil, divMint, 0, 0)
	if result.Severity != "CLEAN" {
		t.Errorf("expected CLEAN for empty txns, got %s", result.Severity)
	}
	if result.Score != 0 {
		t.Errorf("expected score 0, got %d", result.Score)
	}
}
