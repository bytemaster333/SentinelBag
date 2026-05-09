import type { Metadata } from 'next'
import { Inter } from 'next/font/google'
import './globals.css'

const inter = Inter({ subsets: ['latin'] })

export const metadata: Metadata = {
  title: 'SentinelBag — Wash Trading Detector',
  description: 'Detect wash trading patterns on Solana tokens using deterministic on-chain analysis.',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className={`${inter.className} bg-gray-950 text-white min-h-screen antialiased`}>
        {children}
      </body>
    </html>
  )
}
