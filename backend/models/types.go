package models

// WalletShare represents one wallet's contribution to total token volume.
type WalletShare struct {
	Address string  `json:"address"` // first4…last4 truncation
	Share   float64 `json:"share"`   // 0.0–1.0 fraction of total volume
	Volume  float64 `json:"volume"`  // raw token units
}

// CircularPattern is a single detected round-trip flow.
type CircularPattern struct {
	Wallets  []string `json:"wallets"`   // truncated addresses in path order
	HopCount int      `json:"hop_count"` // 2 or 3
}

// EvidenceMetrics carries rule-specific numeric breakdown.
// Fields are omitempty — only the responsible algorithm populates its section.
type EvidenceMetrics struct {
	// Wallet Clustering
	TopWallets   []WalletShare `json:"top_wallets,omitempty"`
	HHI          float64       `json:"hhi,omitempty"`        // Herfindahl–Hirschman Index 0.0–1.0
	Top3Share    float64       `json:"top3_share,omitempty"` // combined share of top 3 senders
	TotalSenders int           `json:"total_senders,omitempty"`

	// Circular Flow
	TwoHopCount   int              `json:"two_hop_count,omitempty"`
	ThreeHopCount int              `json:"three_hop_count,omitempty"`
	TotalPatterns int              `json:"total_patterns,omitempty"`
	Samples       []CircularPattern `json:"samples,omitempty"` // up to 3 example patterns

	// Buyer Diversity
	UniqueWallets  int           `json:"unique_wallets,omitempty"`
	TotalTransfers int           `json:"total_transfers,omitempty"`
	DiversityIndex float64       `json:"diversity_index,omitempty"`
	RepeatBuyers   []WalletShare `json:"repeat_buyers,omitempty"` // top 3 most-seen recipients
}

// AnalysisResult is the enriched output of one heuristic rule.
type AnalysisResult struct {
	Rule     string          `json:"rule"`
	Detail   string          `json:"detail"`
	Severity string          `json:"severity"` // HIGH | MEDIUM | LOW | CLEAN
	Flag     string          `json:"flag"`      // HIGH_CONCENTRATION | CIRCULAR_FLOW | BOT_ACTIVITY | ""
	Score    int             `json:"score"`     // penalty deducted from base 100
	Metrics  EvidenceMetrics `json:"metrics"`
}

// IntegrityScore is the final API response for a token address.
type IntegrityScore struct {
	Token      string           `json:"token"`
	Score      int              `json:"score"`
	Grade      string           `json:"grade"`
	Flags      []string         `json:"flags"`
	Evidence   []AnalysisResult `json:"evidence"`
	SampleSize int              `json:"sample_size"` // number of transactions analysed
	Cached     bool             `json:"cached"`
}

// HeliusTransaction is the shape returned by the Helius enhanced transactions API.
type HeliusTransaction struct {
	Signature       string           `json:"signature"`
	Timestamp       int64            `json:"timestamp"`
	Source          string           `json:"source"`          // originating protocol: "RAYDIUM" | "JUPITER" | "UNKNOWN" | …
	Type            string           `json:"type"`            // transaction type: "SWAP" | "TRANSFER" | …
	FeePayer        string           `json:"feePayer"`
	NativeTransfers []NativeTransfer `json:"nativeTransfers"`
	TokenTransfers  []TokenTransfer  `json:"tokenTransfers"`
}

// NativeTransfer represents a SOL transfer within a transaction.
type NativeTransfer struct {
	FromUserAccount string  `json:"fromUserAccount"`
	ToUserAccount   string  `json:"toUserAccount"`
	Amount          float64 `json:"amount"`
}

// TokenTransfer represents a SPL token transfer within a transaction.
type TokenTransfer struct {
	FromUserAccount string  `json:"fromUserAccount"`
	ToUserAccount   string  `json:"toUserAccount"`
	TokenAmount     float64 `json:"tokenAmount"`
	Mint            string  `json:"mint"`
}

// DASResponse is the JSON-RPC response shape for Helius DAS getTokenAccounts.
type DASResponse struct {
	Result struct {
		TokenAccounts []struct {
			Owner  string  `json:"owner"`
			Amount float64 `json:"amount"`
		} `json:"token_accounts"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}
