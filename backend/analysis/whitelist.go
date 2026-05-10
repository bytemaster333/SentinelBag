package analysis

// infrastructureWallets maps well-known Solana DEX, aggregator, and protocol addresses
// to their human-readable names. These addresses are excluded from sender concentration
// analysis — their volume dominance reflects ecosystem integration, not wash trading.
var infrastructureWallets = map[string]string{
	"675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8": "Raydium AMM v4",
	"CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK": "Raydium CLMM",
	"whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc":  "Orca Whirlpool",
	"LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo": "Meteora DLMM",
	"cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG":  "Meteora DAMM v2",
	"JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4": "Jupiter Aggregator v6",
	"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P":  "Pump.fun",
	"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA":  "Token Program",
	"TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb":  "Token-2022",
}

// IsInfrastructureWallet reports whether address is a well-known DEX, aggregator, or
// protocol address that should be excluded from sender concentration analysis.
// Returns the human-readable label and true if matched; ("", false) otherwise.
func IsInfrastructureWallet(address string) (string, bool) {
	label, ok := infrastructureWallets[address]
	return label, ok
}
