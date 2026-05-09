'use client'

import { useState, FormEvent } from 'react'
import { Search, Loader2 } from 'lucide-react'

interface TokenInputProps {
  onSubmit: (address: string) => void
  isLoading: boolean
}

export function TokenInput({ onSubmit, isLoading }: TokenInputProps) {
  const [value, setValue] = useState('')
  const [validationError, setValidationError] = useState('')

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const trimmed = value.trim()
    setValidationError('')

    // Solana addresses: base58, 32–44 characters
    if (trimmed.length < 32 || trimmed.length > 44) {
      setValidationError('Enter a valid Solana token address (32–44 characters)')
      return
    }

    onSubmit(trimmed)
  }

  return (
    <div className="w-full max-w-2xl mx-auto space-y-2">
      <form onSubmit={handleSubmit}>
        <div
          className={`flex gap-2 items-center bg-gray-900 border rounded-xl px-4 py-3
                      transition-colors focus-within:border-violet-500
                      ${validationError ? 'border-red-500' : 'border-gray-700'}`}
        >
          <Search className="text-gray-400 shrink-0" size={20} />
          <input
            type="text"
            value={value}
            onChange={(e) => {
              setValue(e.target.value)
              if (validationError) setValidationError('')
            }}
            placeholder="Enter Solana token mint address..."
            className="flex-1 bg-transparent outline-none text-white placeholder-gray-500
                       font-mono text-sm"
            disabled={isLoading}
            autoComplete="off"
            spellCheck={false}
          />
          <button
            type="submit"
            disabled={isLoading || !value.trim()}
            className="bg-violet-600 hover:bg-violet-500 disabled:opacity-40 disabled:cursor-not-allowed
                       text-white px-4 py-1.5 rounded-lg text-sm font-semibold
                       transition-colors shrink-0 flex items-center gap-2"
          >
            {isLoading ? (
              <>
                <Loader2 size={14} className="animate-spin" />
                Analyzing…
              </>
            ) : (
              'Analyze'
            )}
          </button>
        </div>
      </form>
      {validationError && (
        <p className="text-red-400 text-xs pl-1">{validationError}</p>
      )}
    </div>
  )
}
