package analysis

import "sentinelbag/models"

// makeTx constructs a minimal HeliusTransaction with a single token transfer.
func makeTx(mint, from, to string, amount float64, ts int64) models.HeliusTransaction {
	return models.HeliusTransaction{
		Timestamp: ts,
		TokenTransfers: []models.TokenTransfer{
			{FromUserAccount: from, ToUserAccount: to, Mint: mint, TokenAmount: amount},
		},
	}
}
