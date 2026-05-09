'use client'

import { useState } from 'react'
import { Shield, ExternalLink } from 'lucide-react'
import { TokenInput } from '@/components/TokenInput'
import { RiskGrade } from '@/components/RiskGrade'
import { EvidenceCard } from '@/components/EvidenceCard'
import { ExampleTokens } from '@/components/ExampleTokens'

interface WalletShare {
  address: string
  share: number
  volume: number
}

interface CircularPattern {
  wallets: string[]
  hop_count: number
}

interface EvidenceMetrics {
  top_wallets?: WalletShare[]
  hhi?: number
  top3_share?: number
  total_senders?: number
  two_hop_count?: number
  three_hop_count?: number
  total_patterns?: number
  samples?: CircularPattern[]
  unique_wallets?: number
  total_transfers?: number
  diversity_index?: number
  repeat_buyers?: WalletShare[]
}

interface AnalysisResult {
  rule: string
  detail: string
  severity: string
  flag: string
  score: number
  metrics: EvidenceMetrics
}

interface IntegrityResponse {
  token: string
  score: number
  grade: string
  flags: string[]
  evidence: AnalysisResult[]
  sample_size: number
  cached: boolean
}

type PageState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'success'; data: IntegrityResponse }
  | { status: 'error'; message: string }

export default function HomePage() {
  const [state, setState] = useState<PageState>({ status: 'idle' })

  async function handleAnalyze(address: string) {
    setState({ status: 'loading' })

    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 25_000)

    try {
      const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/integrity/${address}`, { signal: controller.signal })
      clearTimeout(timeoutId)
      const body = await res.json().catch(() => ({ error: `HTTP ${res.status}` }))

      if (res.status === 404) {
        setState({ status: 'error', message: body.error ?? 'Token not found or no transactions available.' })
        return
      }
      if (res.status === 422) {
        setState({ status: 'error', message: body.error ?? 'Insufficient on-chain data — token may be inactive or newly launched.' })
        return
      }
      if (res.status === 429) {
        setState({ status: 'error', message: 'Helius rate limit reached — please wait a moment and try again.' })
        return
      }
      if (!res.ok) {
        setState({ status: 'error', message: body.error ?? `Unexpected error (${res.status})` })
        return
      }

      setState({ status: 'success', data: body as IntegrityResponse })
    } catch (err) {
      clearTimeout(timeoutId)
      if (err instanceof DOMException && err.name === 'AbortError') {
        setState({ status: 'error', message: 'Request timed out after 25s — the backend may be under load. Please try again.' })
      } else {
        setState({ status: 'error', message: 'Network error — is the backend running on :8080?' })
      }
    }
  }

  return (
    <main className="min-h-screen px-4 py-12">
      <div className="max-w-4xl mx-auto space-y-10">

        {/* Header */}
        <header className="text-center space-y-4">
          <div className="flex items-center justify-center gap-3">
            <Shield className="text-violet-400" size={40} />
            <h1 className="text-5xl font-black tracking-tight bg-gradient-to-r from-violet-400 to-fuchsia-400 bg-clip-text text-transparent">
              SentinelBag
            </h1>
          </div>
          <p className="text-gray-400 text-lg max-w-xl mx-auto">
            Deterministic wash trading detection for Solana tokens.
            Powered by on-chain analysis via Helius.
          </p>
        </header>

        {/* Search */}
        <div className="space-y-3">
          <TokenInput
            onSubmit={handleAnalyze}
            isLoading={state.status === 'loading'}
          />
          <ExampleTokens
            onSelect={handleAnalyze}
            disabled={state.status === 'loading'}
          />
        </div>

        {/* Loading */}
        {state.status === 'loading' && (
          <div className="text-center space-y-5 py-8">
            <div className="w-14 h-14 mx-auto rounded-full border-4 border-violet-500 border-t-transparent animate-spin" />
            <div className="space-y-1">
              <p className="text-gray-300 font-medium">Running analysis…</p>
              <p className="text-gray-500 text-sm">
                Mint-direct + holder fan-out running concurrently → 3 heuristics in parallel
              </p>
            </div>
          </div>
        )}

        {/* Error */}
        {state.status === 'error' && (
          <div className="rounded-xl border border-red-500/40 bg-red-500/10 p-6 text-center">
            <p className="text-red-400 font-semibold">{state.message}</p>
          </div>
        )}

        {/* Results */}
        {state.status === 'success' && (
          <div className="space-y-10">

            {/* Token meta */}
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2 min-w-0">
                <span className="text-gray-500 text-xs font-mono truncate max-w-xs sm:max-w-md">
                  {state.data.token}
                </span>
                <a
                  href={`https://solscan.io/token/${state.data.token}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-gray-600 hover:text-gray-400 transition-colors shrink-0"
                  title="View on Solscan"
                >
                  <ExternalLink size={14} />
                </a>
              </div>
              {state.data.cached && (
                <span className="text-xs bg-gray-800 border border-gray-700 text-gray-400 px-2.5 py-1 rounded-full shrink-0">
                  Cached · refreshes in ~1h
                </span>
              )}
            </div>

            {/* Grade */}
            <div className="flex justify-center">
              <RiskGrade grade={state.data.grade} score={state.data.score} />
            </div>

            {/* Active flags */}
            {state.data.flags.length > 0 && (
              <div className="flex flex-wrap gap-2 justify-center">
                {state.data.flags.map((flag) => (
                  <span
                    key={flag}
                    className="bg-red-500/20 border border-red-500/40 text-red-400
                               text-xs font-mono px-3 py-1 rounded-full"
                  >
                    ⚠ {flag}
                  </span>
                ))}
              </div>
            )}

            {/* Evidence cards */}
            <div className="space-y-3">
              <h2 className="text-lg font-semibold text-gray-200">Analysis Breakdown</h2>
              <div className="grid gap-4 md:grid-cols-3">
                {state.data.evidence.map((ev) => (
                  <EvidenceCard key={ev.rule} evidence={ev} />
                ))}
              </div>
            </div>

            {/* Score formula */}
            <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5 text-sm text-gray-500 space-y-1">
              <p className="text-gray-400 font-medium mb-2">Score breakdown</p>
              <p>Base: <span className="text-gray-300 font-mono">100</span></p>
              {state.data.evidence.map((ev) =>
                ev.score > 0 ? (
                  <p key={ev.rule}>
                    {ev.rule}:{' '}
                    <span className="text-red-400 font-mono">−{ev.score}</span>
                    <span className="ml-1 text-gray-600">({ev.severity})</span>
                  </p>
                ) : null
              )}
              <p className="border-t border-gray-800 pt-2 mt-2 text-gray-300 font-mono">
                = {state.data.score} / 100 → Grade {state.data.grade}
              </p>
            </div>
          </div>
        )}
      </div>
    </main>
  )
}
