'use client'

import {
  Users, RefreshCw, BarChart3,
  AlertTriangle, AlertCircle, Info, CheckCircle2,
  TrendingUp,
} from 'lucide-react'

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
  // Clustering
  top_wallets?: WalletShare[]
  hhi?: number
  top3_share?: number
  total_senders?: number
  // Circular
  two_hop_count?: number
  three_hop_count?: number
  total_patterns?: number
  samples?: CircularPattern[]
  // Diversity
  unique_wallets?: number
  total_transfers?: number
  diversity_index?: number
  repeat_buyers?: WalletShare[]
}

interface Evidence {
  rule: string
  detail: string
  severity: string
  flag: string
  score: number
  metrics: EvidenceMetrics
}

const severityStyle: Record<string, {
  border: string; bg: string; text: string
  badge: string; barFill: string
}> = {
  HIGH:   { border: 'border-red-500/40',    bg: 'bg-red-500/10',    text: 'text-red-400',    badge: 'bg-red-500/20 text-red-400 border-red-500/30',    barFill: 'bg-red-500'    },
  MEDIUM: { border: 'border-amber-500/40',  bg: 'bg-amber-500/10',  text: 'text-amber-400',  badge: 'bg-amber-500/20 text-amber-400 border-amber-500/30',  barFill: 'bg-amber-400'  },
  LOW:    { border: 'border-blue-500/40',   bg: 'bg-blue-500/10',   text: 'text-blue-400',   badge: 'bg-blue-500/20 text-blue-400 border-blue-500/30',   barFill: 'bg-blue-400'   },
  CLEAN:  { border: 'border-green-500/30',  bg: 'bg-green-500/5',   text: 'text-green-400',  badge: 'bg-green-500/20 text-green-400 border-green-500/30',  barFill: 'bg-green-500'  },
}

const ruleIcon: Record<string, React.ElementType> = {
  'Wallet Clustering': Users,
  'Circular Flow':     RefreshCw,
  'Buyer Diversity':   BarChart3,
}

const severityIcon: Record<string, React.ElementType> = {
  HIGH:   AlertTriangle,
  MEDIUM: AlertCircle,
  LOW:    Info,
  CLEAN:  CheckCircle2,
}

// ── Metric sub-components ─────────────────────────────────────────────────────

function WalletBar({ wallet, barFill }: { wallet: WalletShare; barFill: string }) {
  return (
    <div className="space-y-0.5">
      <div className="flex justify-between text-xs text-gray-400">
        <span className="font-mono">{wallet.address}</span>
        <span>{(wallet.share * 100).toFixed(1)}%</span>
      </div>
      <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full ${barFill}`}
          style={{ width: `${Math.min(wallet.share * 100, 100)}%` }}
        />
      </div>
    </div>
  )
}

function ClusteringMetrics({ m, barFill }: { m: EvidenceMetrics; barFill: string }) {
  if (!m.top_wallets?.length) return null
  return (
    <div className="space-y-2">
      <div className="flex gap-4 text-xs text-gray-500">
        {m.hhi !== undefined && (
          <span>HHI: <span className="text-gray-300 font-mono">{m.hhi.toFixed(3)}</span></span>
        )}
        {m.top3_share !== undefined && (
          <span>Top-3 share: <span className="text-gray-300 font-mono">{(m.top3_share * 100).toFixed(0)}%</span></span>
        )}
        {m.total_senders !== undefined && (
          <span>Senders: <span className="text-gray-300 font-mono">{m.total_senders}</span></span>
        )}
      </div>
      <div className="space-y-1.5">
        {m.top_wallets.map((w) => (
          <WalletBar key={w.address} wallet={w} barFill={barFill} />
        ))}
      </div>
    </div>
  )
}

function CircularMetrics({ m }: { m: EvidenceMetrics }) {
  if (m.total_patterns === undefined) return null
  return (
    <div className="space-y-2">
      <div className="flex gap-4 text-xs text-gray-500">
        <span>2-hop: <span className="text-gray-300 font-mono">{m.two_hop_count ?? 0}</span></span>
        <span>3-hop: <span className="text-gray-300 font-mono">{m.three_hop_count ?? 0}</span></span>
        <span>Total: <span className="text-gray-300 font-mono">{m.total_patterns}</span></span>
      </div>
      {m.samples && m.samples.length > 0 && (
        <div className="space-y-1">
          <p className="text-xs text-gray-600">Example patterns:</p>
          {m.samples.map((s, i) => (
            <div key={i} className="flex items-center gap-1 text-xs font-mono text-gray-400">
              {s.wallets.map((w, j) => (
                <span key={j} className="flex items-center gap-1">
                  <span className="bg-gray-800 px-1.5 py-0.5 rounded">{w}</span>
                  {j < s.wallets.length - 1 && <span className="text-gray-600">→</span>}
                </span>
              ))}
              <span className="text-gray-600 ml-1">→ {s.wallets[0]}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function DiversityMetrics({ m, barFill }: { m: EvidenceMetrics; barFill: string }) {
  if (m.diversity_index === undefined) return null
  return (
    <div className="space-y-2">
      <div className="flex gap-4 text-xs text-gray-500">
        {m.unique_wallets !== undefined && (
          <span>Unique: <span className="text-gray-300 font-mono">{m.unique_wallets}</span></span>
        )}
        {m.total_transfers !== undefined && (
          <span>Transfers: <span className="text-gray-300 font-mono">{m.total_transfers}</span></span>
        )}
      </div>
      {/* BDI gauge */}
      <div className="space-y-0.5">
        <div className="flex justify-between text-xs text-gray-500">
          <span>Diversity Index</span>
          <span className="text-gray-300 font-mono">{m.diversity_index.toFixed(3)}</span>
        </div>
        <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
          <div
            className={`h-full rounded-full transition-all ${barFill}`}
            style={{ width: `${Math.min(m.diversity_index * 100, 100)}%` }}
          />
        </div>
        <div className="flex justify-between text-xs text-gray-700">
          <span>0.0 — bot</span>
          <span>1.0 — organic</span>
        </div>
      </div>
      {m.repeat_buyers && m.repeat_buyers.length > 0 && (
        <div className="space-y-1.5">
          <p className="text-xs text-gray-600">Top repeat recipients:</p>
          {m.repeat_buyers.map((w) => (
            <WalletBar key={w.address} wallet={{ ...w, share: w.share }} barFill={barFill} />
          ))}
        </div>
      )}
    </div>
  )
}

// ── Main component ────────────────────────────────────────────────────────────

export function EvidenceCard({ evidence }: { evidence: Evidence }) {
  const sev = evidence.severity || 'CLEAN'
  const style = severityStyle[sev] ?? severityStyle['CLEAN']
  const RuleIcon = ruleIcon[evidence.rule] ?? TrendingUp
  const SeverityIcon = severityIcon[sev] ?? Info

  return (
    <div className={`rounded-xl border ${style.border} ${style.bg} p-5 space-y-4`}>
      {/* Header */}
      <div className="flex items-center gap-2">
        <RuleIcon className={`${style.text} shrink-0`} size={18} />
        <span className={`font-semibold text-sm ${style.text}`}>{evidence.rule}</span>
        <span className={`ml-auto text-xs px-2 py-0.5 rounded-full border font-mono ${style.badge}`}>
          {sev}
        </span>
      </div>

      {/* Summary detail */}
      <p className="text-gray-300 text-sm leading-relaxed">{evidence.detail}</p>

      {/* Rule-specific metrics */}
      {evidence.rule === 'Wallet Clustering' && (
        <ClusteringMetrics m={evidence.metrics} barFill={style.barFill} />
      )}
      {evidence.rule === 'Circular Flow' && (
        <CircularMetrics m={evidence.metrics} />
      )}
      {evidence.rule === 'Buyer Diversity' && (
        <DiversityMetrics m={evidence.metrics} barFill={style.barFill} />
      )}

      {/* Footer: flag + penalty */}
      <div className="flex items-center justify-between pt-1 border-t border-white/5">
        {evidence.flag ? (
          <span className="text-xs font-mono text-gray-500 bg-gray-900 px-2 py-0.5 rounded">
            {evidence.flag}
          </span>
        ) : (
          <span />
        )}
        <div className={`flex items-center gap-1 text-xs ${style.text}`}>
          <SeverityIcon size={12} />
          <span>−{evidence.score} pts</span>
        </div>
      </div>
    </div>
  )
}
