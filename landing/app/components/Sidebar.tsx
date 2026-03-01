'use client'

import { useState } from 'react'
import { useLang } from '@/app/context/LangContext'

const indicators = ['hero', 'features', 'screenshots', 'get-saturx']

export default function Sidebar() {
  const [active, setActive] = useState(0)
  const { t } = useLang()

  return (
    <aside className="w-14 shrink-0 hidden lg:flex flex-col items-center gap-8 py-8 border-r border-white/10 bg-gradient-to-b from-saturx-purple/40 to-transparent">
      <div className="flex flex-col items-center gap-6">
        <span
          className="text-[10px] font-medium text-white/70 tracking-widest uppercase whitespace-nowrap"
          style={{ writingMode: 'vertical-rl', transform: 'rotate(180deg)' }}
        >
          {t.sidebar.tagline}
        </span>
        <span
          className="text-xs font-semibold text-white/90 tracking-wider whitespace-nowrap"
          style={{ writingMode: 'vertical-rl', transform: 'rotate(180deg)' }}
        >
          {t.sidebar.brand}
        </span>
      </div>
      <div className="flex flex-col gap-2">
        {indicators.map((id, i) => (
          <a
            key={id}
            href={`#${id}`}
            onClick={() => setActive(i)}
            className={`w-2 h-2 rounded-full transition-colors ${
              active === i ? 'bg-white' : 'bg-white/40 hover:bg-white/60'
            }`}
            aria-label={`Go to section ${id}`}
          />
        ))}
      </div>
    </aside>
  )
}
