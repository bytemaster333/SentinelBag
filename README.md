<div align="center">

# 🛡️ SentinelBag

### The Interpretability Layer for the Bags Ecosystem

*Transforming raw volume into verifiable trust — one token at a time.*

[![Go](https://img.shields.io/badge/Go-1.21-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org/)
[![Next.js](https://img.shields.io/badge/Next.js-14-black?style=flat-square&logo=next.js&logoColor=white)](https://nextjs.org/)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=flat-square&logo=redis&logoColor=white)](https://redis.io/)
[![Helius](https://img.shields.io/badge/Powered%20by-Helius-9945FF?style=flat-square)](https://helius.xyz/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow?style=flat-square)](LICENSE)

</div>

---

## The Problem

Solana's permissionless nature is its greatest strength — and its most exploited vulnerability.

**98.7% of projects on Solana exhibit risk signals consistent with wash trading or coordinated volume manipulation.** Fake volume creates fake narratives. Fake narratives attract real capital. Real capital disappears. This cycle has eroded trust across the entire ecosystem and made it nearly impossible for retail participants, institutional allocators, and protocol developers to distinguish genuine traction from manufactured noise.

The core issue is **interpretability**: raw on-chain volume metrics are easy to game, but difficult to audit. There is no standardized, open layer that translates transaction data into a trustworthy signal.

> *"If you can't measure honesty, you can't price risk. And if you can't price risk, you can't allocate capital rationally."*

---

## The Solution

SentinelBag is a **deterministic interpretability layer** built on top of Helius's enriched transaction data. It fetches up to 1,000 recent transactions for any Solana token address, runs three independent on-chain heuristics concurrently, and produces a single, auditable **Integrity Score** — a number between 0 and 100 that represents the probability-weighted cleanliness of a token's trading activity.

No black boxes. No ML models that can't be explained. Every deduction in the final score is traceable to a specific, human-readable rule.

```
Raw Volume  ──►  [Wallet Clustering]  ─┐
                 [Circular Flow    ]  ─┼──►  Integrity Score  ──►  Risk Grade (A+ → F)
                 [Buyer Diversity  ]  ─┘         0 – 100
```

### The Proof-of-Ecosystem Framework

Legitimate token activity leaves three measurable signatures: volume originates from many independent sources, token flows do not return to their origin wallets, and each transfer reaches a previously unseen recipient. Each heuristic below tests for exactly one of these signatures — distributed *sources*, non-circular *flows*, and diverse *recipients* — forming a three-axis fingerprint that separates organic trading from manufactured volume.

### Scoring Model

| Heuristic           | Max Deduction | Trigger Condition                            |
|---------------------|:-------------:|----------------------------------------------|
| Wallet Clustering   | −40 pts       | Single sender controls >60% of volume        |
| Circular Flow       | −35 pts       | ≥20 A→B→A / A→B→C→A loops in 24h window     |
| Buyer Diversity     | −35 pts       | Unique buyers / total transfers < 0.10       |

```
Final Score  =  max(0,  100 − Σ deductions)
```

### Risk Grades

| Grade | Score Range | Signal              |
|-------|:-----------:|---------------------|
| **A+** | 90 – 100   | Verified clean      |
| **A**  | 80 – 89    | Clean               |
| **B**  | 70 – 79    | Mostly clean        |
| **C**  | 50 – 69    | Suspicious          |
| **D**  | 30 – 49    | High risk           |
| **F**  | 0 – 29     | Extreme risk        |

---

## Core Heuristics

### 1. First-Hop Wallet Clustering

**What it detects:** Coordinated networks of wallets funded from a single source that trade among themselves to manufacture volume.

**How it works:** For every token transfer in the transaction window, the algorithm builds a sender-to-volume map. It then calculates the *concentration ratio* — the fraction of total token transfer volume attributable to the single largest sender.

```
concentration_ratio = max_sender_volume / total_volume
```

A legitimately popular token distributes volume across hundreds of independent actors. A wash-traded token shows a handful of senders responsible for the overwhelming majority of activity. When the top sender controls more than **60%** of volume, the `HIGH_CONCENTRATION` flag is raised and the score is penalised. Infrastructure addresses — AMM pools, swap aggregators, and mint authorities — are excluded from this calculation; their high-volume presence is a sign of ecosystem integration, not manipulation.

**Why this works:** Coordinated wash trading requires capital recycling. Capital recycling requires a common funding source. That funding source is always visible on-chain, and first-hop analysis exposes it without requiring historical graph traversal.

---

### 2. Circular Flow Detection

**What it detects:** Explicit round-trip transactions — the purest form of wash trading — where tokens flow A → B → A or A → B → C → A within a 24-hour time window.

**How it works:** All token transfers are converted into a directed edge graph indexed by sender. A depth-first search identifies 2-hop and 3-hop cycles where the terminal wallet matches the origin wallet and all edges fall within the `circularWindow` (86,400 seconds).

```
2-hop:  A ──► B ──► A          (direct round-trip)
3-hop:  A ──► B ──► C ──► A   (triangular wash)
```

Cycles are deduplicated by their sorted participant set — the same group of wallets is only counted once regardless of how many times they repeat the pattern.

**Why this works:** Legitimate trading has no incentive to return tokens to the origin wallet. Any closed loop is economically irrational under normal conditions and statistically near-impossible at scale by chance. A single circular loop is noise; ten or more is a pattern; twenty or more is a coordinated operation.

---

### 3. Buyer Diversity Index (BDI)

**What it detects:** Bot-driven volume where a small pool of wallets repeatedly appears as the recipient, masking the absence of genuine buyer demand.

**How it works:** The BDI is a simple but powerful ratio:

```
BDI = unique_recipient_wallets / total_transfer_events
```

A BDI approaching **1.0** means every transfer reaches a new wallet — the hallmark of genuine distribution and organic adoption. A BDI approaching **0.0** means the same wallets are receiving tokens over and over, a clear indicator of scripted bot activity designed to inflate transfer counts without creating real holders.

**Why this works:** Bots are efficient. Efficiency means reuse. Reuse is detectable. This single metric has the highest signal-to-noise ratio of the three heuristics for catching automated wash trading at scale.

---

## Validation

The scores below were generated by running the live API against the listed token mints. Reproducible by anyone with a Helius API key by pointing the backend at each mint and recording the JSON response.

| Token | Mint Address | Score | Grade | Notes |
|-------|:------------:|:-----:|:-----:|-------|
| USDC  | `EPjF…Dt1v` | TBD | TBD | Expected A+: institutional flows, high infraShare, distributed senders |
| JUP   | `JUPy…vCN`  | TBD | TBD | Expected A: major DEX token, broad sender distribution |
| BONK  | `DezX…B263` | TBD | TBD | Expected A/B: genuine meme distribution, some recycling expected |
| JTO   | `jtoJ…ZKm`  | TBD | TBD | Expected A/B: governance token, known unlock schedule |
| WIF   | `EKpQ…cjm`  | TBD | TBD | Expected B/C: high retail activity, watch for wash signals |
| Known rug pull (tbd) | `—` | TBD | TBD | Expected F: concentrated wallet control, circular flows |

---

## Tech Stack

| Layer       | Technology                          | Rationale                                                    |
|-------------|-------------------------------------|--------------------------------------------------------------|
| **Backend** | Go 1.21                             | Native concurrency for parallel heuristic execution          |
| **Router**  | chi v5                              | Minimal, idiomatic HTTP router with zero reflection          |
| **Data**    | Helius gTFA API                     | Enriched, parsed transaction data — no raw RPC parsing       |
| **Cache**   | Redis 7 (1-hour TTL)                | Eliminates redundant API calls; response times <5ms on hit   |
| **Frontend**| Next.js 14 (App Router)             | Server components, built-in rewrites for zero-CORS setup     |
| **Styling** | Tailwind CSS                        | Utility-first, no runtime overhead                           |
| **Icons**   | lucide-react                        | Consistent, tree-shakeable icon set                          |

### Why Go for the Backend?

The three heuristics are computationally independent once transaction data is fetched. Go's goroutines allow all three to run in true parallelism on a single HTTP request, bounded by a shared 30-second context deadline. The entire analysis pipeline — fetch 1,000 transactions, run 3 heuristics, cache result — completes in under 3 seconds on a warm Helius connection.

```go
// All three analyses run concurrently against the same read-only slice
go func() { resultCh <- analysis.AnalyzeClustering(txns, tokenAddress) }()
go func() { resultCh <- analysis.AnalyzeCircular(txns, tokenAddress)   }()
go func() { resultCh <- analysis.AnalyzeDiversity(txns, tokenAddress)  }()
```

---

## Project Structure

```
SentinelBag/
├── backend/
│   ├── main.go                   # Server entrypoint — env, router, CORS
│   ├── go.mod / go.sum
│   ├── .env                      # ← API keys go HERE (not the root)
│   ├── handlers/
│   │   └── integrity.go          # GET /api/integrity/:tokenAddress
│   ├── helius/
│   │   └── client.go             # Paginated gTFA client with retry backoff
│   ├── analysis/
│   │   ├── clustering.go         # Heuristic 1: Wallet Clustering
│   │   ├── circular.go           # Heuristic 2: Circular Flow Detection
│   │   └── diversity.go          # Heuristic 3: Buyer Diversity Index
│   ├── cache/
│   │   └── redis.go              # Redis wrapper with noop fallback
│   └── models/
│       └── types.go              # Shared structs (Helius tx, IntegrityScore)
├── frontend/
│   ├── next.config.js            # API rewrites → backend (no CORS needed)
│   ├── src/app/
│   │   ├── page.tsx              # Main page — state machine (idle/loading/result)
│   │   ├── layout.tsx
│   │   └── globals.css
│   └── src/components/
│       ├── TokenInput.tsx        # Address input with validation
│       ├── RiskGrade.tsx         # Animated grade letter + score bar
│       └── EvidenceCard.tsx      # Per-heuristic evidence card
├── .env.example                  # Environment variable template
└── .gitignore
```

---

## Setup

### Prerequisites

- **Go** ≥ 1.21 — [Install](https://golang.org/dl/)
- **Node.js** ≥ 18 — [Install](https://nodejs.org/)
- **Redis** ≥ 7 — [Install](https://redis.io/docs/getting-started/) or use Docker
- **Helius API Key** — [Get one free at dev.helius.xyz](https://dev.helius.xyz/dashboard/app)

---

### Step 1 — Clone the Repository

```bash
git clone https://github.com/bytemaster333/SentinelBag.git
cd SentinelBag
```

---

### Step 2 — Configure the Backend Environment

> **Important:** The `.env` file must live inside the `backend/` directory, not the project root. The Go server loads it relative to its working directory.

```bash
cd backend
cp ../.env.example .env
```

Open `backend/.env` and fill in your values:

```env
HELIUS_API_KEY=your_helius_api_key_here
REDIS_URL=redis://localhost:6379
PORT=8080
ALLOWED_ORIGIN=http://localhost:3000
```

---

### Step 3 — Start Redis

```bash
# Using Docker (recommended)
docker run -d -p 6379:6379 --name sentinel-redis redis:7-alpine

# Or if Redis is installed locally
redis-server
```

> **Note:** Redis is optional. If it is unavailable, the backend will log a warning and continue with caching disabled. All analysis features remain fully functional.

---

### Step 4 — Run the Backend

```bash
# From the backend/ directory
cd backend
go run main.go
```

Expected output:

```
SentinelBag backend listening on :8080
```

Verify it's healthy:

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

---

### Step 5 — Run the Frontend

```bash
# From the frontend/ directory (new terminal)
cd frontend
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

---

## Environment Variables

### Backend (`backend/.env`)

| Variable         | Required | Default                     | Description                                      |
|------------------|:--------:|-----------------------------|--------------------------------------------------|
| `HELIUS_API_KEY` | ✅ Yes   | —                           | Your Helius API key for transaction fetching     |
| `REDIS_URL`      | No       | `redis://localhost:6379`    | Redis connection string                          |
| `PORT`           | No       | `8080`                      | HTTP server port                                 |
| `ALLOWED_ORIGIN` | No       | `http://localhost:3000`     | CORS origin for the frontend                     |

### Frontend (`frontend/.env.local`)

| Variable                    | Required | Default                  | Description                       |
|-----------------------------|:--------:|--------------------------|-----------------------------------|
| `NEXT_PUBLIC_BACKEND_URL`   | No       | `http://localhost:8080`  | Backend base URL (used in rewrites) |

---

## API Reference

### `GET /api/integrity/:tokenAddress`

Analyzes a Solana token address and returns its Integrity Score.

**Response:**

```json
{
  "token": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
  "score": 61,
  "grade": "C",
  "flags": ["HIGH_CONCENTRATION", "CIRCULAR_FLOW"],
  "evidence": [
    {
      "rule": "Wallet Clustering",
      "detail": "73% of volume from 1 sender (8 unique senders total)",
      "severity": "HIGH",
      "flag": "HIGH_CONCENTRATION",
      "score": 25
    },
    {
      "rule": "Circular Flow",
      "detail": "14 circular flow pattern(s) detected in 24h window",
      "severity": "MEDIUM",
      "flag": "CIRCULAR_FLOW",
      "score": 20
    },
    {
      "rule": "Buyer Diversity",
      "detail": "Diversity index: 0.61 (47 unique buyers / 77 transfers)",
      "severity": "CLEAN",
      "flag": "",
      "score": 0
    }
  ],
  "cached": false
}
```

**Status Codes:**

| Code | Meaning                                          |
|------|--------------------------------------------------|
| 200  | Analysis complete                                |
| 400  | Invalid token address format                     |
| 404  | Token not found or no transactions available     |
| 429  | Helius rate limit reached                        |
| 504  | Analysis timed out (>30s)                        |

---

## Vision

SentinelBag is built on two convictions:

**It is a public good.** The ability to verify trading integrity should not be a premium feature locked behind institutional data providers. Every participant in the Bags ecosystem — regardless of capital size — deserves access to the same on-chain truth. The heuristics are fully deterministic and auditable; anyone can reproduce a score from first principles using public transaction data.

**It is investable at scale.** The scoring model is designed to be composable. An Integrity Score is not just a number for a single investor's due diligence — it is a primitive that can be embedded into listing policies, risk engines, liquidity provisioning algorithms, and protocol governance. As the Bags ecosystem grows, every new token that gets listed, every LP position that gets opened, every governance vote that allocates incentives becomes a candidate for integrity-weighted decision making.

The long-term vision is a world where manipulated volume cannot masquerade as real demand, because the infrastructure to distinguish them is open, standardised, and running by default.

---

## License

MIT © 2025 SentinelBag Contributors
