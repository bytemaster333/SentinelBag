package analysis

import (
	"testing"

	"sentinelbag/models"
)

const circMint = "CIRCULARTESTmint111111111111111111111111111"

func TestCircular_NoLoops(t *testing.T) {
	// Linear A→B→C→D chain — no wallet returns to its origin.
	txns := []models.HeliusTransaction{
		makeTx(circMint, "walletA", "walletB", 1.0, 1000),
		makeTx(circMint, "walletB", "walletC", 1.0, 2000),
		makeTx(circMint, "walletC", "walletD", 1.0, 3000),
	}
	result := AnalyzeCircular(txns, circMint, 0, 3)
	if result.Severity != "CLEAN" {
		t.Errorf("expected CLEAN for linear chain, got %s (score %d): %s",
			result.Severity, result.Score, result.Detail)
	}
	if result.Metrics.TotalPatterns != 0 {
		t.Errorf("expected 0 patterns, got %d", result.Metrics.TotalPatterns)
	}
}

func TestCircular_TwoHopLoop(t *testing.T) {
	// 5 unique A→B→A pairs — each is a distinct 2-hop circular flow.
	pairs := [][2]string{
		{"alice1", "bob1"}, {"alice2", "bob2"}, {"alice3", "bob3"},
		{"alice4", "bob4"}, {"alice5", "bob5"},
	}
	var txns []models.HeliusTransaction
	ts := int64(1000)
	for _, p := range pairs {
		txns = append(txns, makeTx(circMint, p[0], p[1], 1.0, ts))
		txns = append(txns, makeTx(circMint, p[1], p[0], 1.0, ts+3600))
		ts += 10000
	}
	result := AnalyzeCircular(txns, circMint, 0, 10)
	if result.Severity == "CLEAN" && result.Score == 0 {
		t.Errorf("expected circular patterns detected, got CLEAN: %s", result.Detail)
	}
	if result.Metrics.TwoHopCount == 0 {
		t.Errorf("expected two-hop count > 0, got 0")
	}
}

func TestCircular_OutsideWindow(t *testing.T) {
	// A→B at t=1000, B→A at t=1000+86401 — just outside the 24h window.
	// Should not be counted as a circular pattern.
	txns := []models.HeliusTransaction{
		makeTx(circMint, "walletA", "walletB", 1.0, 1000),
		makeTx(circMint, "walletB", "walletA", 1.0, 1000+86401),
	}
	result := AnalyzeCircular(txns, circMint, 0, 2)
	if result.Metrics.TotalPatterns != 0 {
		t.Errorf("expected 0 patterns (outside window), got %d: %s",
			result.Metrics.TotalPatterns, result.Detail)
	}
	if result.Severity != "CLEAN" {
		t.Errorf("expected CLEAN, got %s", result.Severity)
	}
}

func TestCircular_ThreeHopLoop(t *testing.T) {
	// A→B→C→A within a 200-second window — a 3-hop circular pattern.
	txns := []models.HeliusTransaction{
		makeTx(circMint, "nodeA", "nodeB", 1.0, 100),
		makeTx(circMint, "nodeB", "nodeC", 1.0, 150),
		makeTx(circMint, "nodeC", "nodeA", 1.0, 200),
	}
	result := AnalyzeCircular(txns, circMint, 0, 3)
	if result.Metrics.ThreeHopCount == 0 {
		t.Errorf("expected three-hop count > 0, got 0: %s", result.Detail)
	}
	if result.Severity == "CLEAN" && result.Score == 0 {
		t.Errorf("expected non-zero penalty for triangular loop, got CLEAN: %s", result.Detail)
	}
}
