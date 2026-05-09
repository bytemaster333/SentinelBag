'use client'

import { useState } from 'react'
import { Copy, Check, ArrowRight } from 'lucide-react'

interface CaseStudy {
  symbol: string
  name: string
  address: string
  description: string
  expectedGrade: string
  expectedLabel: string
  why: string
  theme: {
    accent: string       // gradient top bar
    border: string       // card border
    hoverBorder: string  // border on hover
    bg: string           // card background
    hoverBg: string      // background on hover
    text: string         // symbol color
    badge: string        // grade badge classes
  }
}

const CASE_STUDIES: CaseStudy[] = [
  {
    symbol: 'USDC',
    name: 'USD Coin',
    address: 'EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v',
    description: 'Institutional stablecoin',
    expectedGrade: 'A+',
    expectedLabel: 'Expected: Clean',
    why: 'High-volume institutional flows; DEX/CEX whitelisted from concentration penalty.',
    theme: {
      accent:      'bg-gradient-to-r from-blue-600 to-blue-400',
      border:      'border-blue-900/40',
      hoverBorder: 'hover:border-blue-500/60',
      bg:          'bg-blue-950/20',
      hoverBg:     'hover:bg-blue-950/40',
      text:        'text-blue-300',
      badge:       'bg-blue-500/20 text-blue-300 border-blue-500/40',
    },
  },
  {
    symbol: 'JUP',
    name: 'Jupiter',
    address: 'JUPyiwrYJFskUPiHa7hkeR8VUtAeFoSYbKedZNsDvCN',
    description: 'DEX aggregator governance',
    expectedGrade: 'A',
    expectedLabel: 'Expected: Clean',
    why: 'Diverse holder base; Jupiter program itself is whitelisted infrastructure.',
    theme: {
      accent:      'bg-gradient-to-r from-violet-600 to-violet-400',
      border:      'border-violet-900/40',
      hoverBorder: 'hover:border-violet-500/60',
      bg:          'bg-violet-950/20',
      hoverBg:     'hover:bg-violet-950/40',
      text:        'text-violet-300',
      badge:       'bg-violet-500/20 text-violet-300 border-violet-500/40',
    },
  },
  {
    symbol: 'BONK',
    name: 'Bonk',
    address: 'DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263',
    description: 'High-volume meme coin',
    expectedGrade: '?',
    expectedLabel: 'Stress Test',
    why: 'Meme coins often show mixed signals — test the detector and see where it lands.',
    theme: {
      accent:      'bg-gradient-to-r from-amber-600 to-orange-400',
      border:      'border-amber-900/40',
      hoverBorder: 'hover:border-amber-500/60',
      bg:          'bg-amber-950/20',
      hoverBg:     'hover:bg-amber-950/40',
      text:        'text-amber-300',
      badge:       'bg-amber-500/20 text-amber-300 border-amber-500/40',
    },
  },
]

function CopyButton({ address }: { address: string }) {
  const [copied, setCopied] = useState(false)

  function handleCopy(e: React.MouseEvent) {
    e.stopPropagation()
    navigator.clipboard.writeText(address).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    })
  }

  return (
    <button
      onClick={handleCopy}
      title="Copy full address"
      className="shrink-0 text-gray-600 hover:text-gray-300 transition-colors p-0.5 rounded"
    >
      {copied ? <Check size={12} className="text-green-400" /> : <Copy size={12} />}
    </button>
  )
}

interface ExampleTokensProps {
  onSelect: (address: string) => void
  disabled?: boolean
}

export function ExampleTokens({ onSelect, disabled }: ExampleTokensProps) {
  return (
    <div className="w-full max-w-2xl mx-auto space-y-2">
      <p className="text-xs text-gray-600 uppercase tracking-wider pl-0.5">
        Case studies — click to analyze
      </p>
      <div className="grid grid-cols-3 gap-3">
        {CASE_STUDIES.map((cs) => (
          <div
            key={cs.address}
            onClick={() => !disabled && onSelect(cs.address)}
            className={`
              group relative rounded-xl border overflow-hidden
              transition-all duration-200 cursor-pointer select-none
              ${cs.theme.border} ${cs.theme.hoverBorder}
              ${cs.theme.bg} ${cs.theme.hoverBg}
              ${disabled ? 'opacity-40 pointer-events-none' : ''}
            `}
          >
            {/* Color accent bar */}
            <div className={`h-0.5 w-full ${cs.theme.accent}`} />

            <div className="p-3 space-y-2.5">
              {/* Symbol + grade badge */}
              <div className="flex items-start justify-between gap-1">
                <span className={`text-xl font-black leading-none ${cs.theme.text}`}>
                  {cs.symbol}
                </span>
                <span
                  className={`text-xs px-1.5 py-0.5 rounded-full border font-mono shrink-0 ${cs.theme.badge}`}
                >
                  {cs.expectedGrade}
                </span>
              </div>

              {/* Name + expected label */}
              <div>
                <p className="text-xs font-semibold text-gray-300 leading-tight">{cs.name}</p>
                <p className="text-xs text-gray-500 leading-tight">{cs.description}</p>
              </div>

              {/* Why */}
              <p className="text-xs text-gray-600 leading-relaxed line-clamp-2">{cs.why}</p>

              {/* Address + copy */}
              <div
                className="flex items-center gap-1.5 pt-1.5 border-t border-white/5"
                onClick={(e) => e.stopPropagation()}
              >
                <span className="text-xs font-mono text-gray-700 flex-1 truncate">
                  {cs.address.slice(0, 4)}…{cs.address.slice(-4)}
                </span>
                <CopyButton address={cs.address} />
              </div>

              {/* Analyze CTA */}
              <div
                className={`
                  flex items-center justify-center gap-1 w-full py-1.5 rounded-lg
                  text-xs font-medium transition-all
                  ${cs.theme.text} bg-white/5 group-hover:bg-white/10
                `}
              >
                Analyze
                <ArrowRight size={11} className="transition-transform group-hover:translate-x-0.5" />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
