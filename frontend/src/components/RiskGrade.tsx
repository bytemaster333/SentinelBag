'use client'

interface RiskGradeProps {
  grade: string
  score: number
}

const gradeConfig: Record<string, { ring: string; text: string; bar: string; label: string }> = {
  'A+': { ring: 'border-green-500',  text: 'text-green-400',  bar: 'bg-green-500',  label: 'Very Clean'   },
  'A':  { ring: 'border-green-500',  text: 'text-green-400',  bar: 'bg-green-500',  label: 'Clean'        },
  'B':  { ring: 'border-emerald-500',text: 'text-emerald-400',bar: 'bg-emerald-500',label: 'Mostly Clean' },
  'C':  { ring: 'border-amber-400',  text: 'text-amber-400',  bar: 'bg-amber-400',  label: 'Suspicious'   },
  'D':  { ring: 'border-orange-500', text: 'text-orange-400', bar: 'bg-orange-500', label: 'High Risk'     },
  'F':  { ring: 'border-red-500',    text: 'text-red-500',    bar: 'bg-red-500',    label: 'Extreme Risk'  },
}

const fallback = gradeConfig['F']

export function RiskGrade({ grade, score }: RiskGradeProps) {
  const cfg = gradeConfig[grade] ?? fallback

  return (
    <div className="flex flex-col items-center gap-6">
      {/* Grade letter */}
      <div
        className={`grade-reveal w-44 h-44 rounded-3xl border-4 ${cfg.ring}
                    bg-gray-900 flex items-center justify-center shadow-2xl`}
      >
        <span className={`text-8xl font-black leading-none ${cfg.text}`}>
          {grade}
        </span>
      </div>

      {/* Label + score */}
      <div className="text-center space-y-1">
        <p className={`text-2xl font-bold ${cfg.text}`}>{cfg.label}</p>
        <p className="text-gray-400 text-sm">Integrity Score: {score} / 100</p>
      </div>

      {/* Score bar */}
      <div className="w-64 h-2 bg-gray-800 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-700 ${cfg.bar}`}
          style={{ width: `${score}%` }}
        />
      </div>
    </div>
  )
}
