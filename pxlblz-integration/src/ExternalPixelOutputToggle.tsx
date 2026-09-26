import { useState } from 'react'
import { RadioTower } from 'lucide-react'
import {
  externalPixelOutputPreference,
  externalPixelOutputUrlPreference,
  setExternalPixelOutputPreference,
} from '@/engine/externalPixelOutput'

export function ExternalPixelOutputToggle() {
  const [enabled] = useState(() => externalPixelOutputPreference())
  const url = externalPixelOutputUrlPreference()

  const toggle = () => {
    const next = !enabled
    setExternalPixelOutputPreference(next)

    // Rebuild the Preview/render loop through the normal app startup path.
    // Keeping the explicit URL marker also makes the active state inspectable
    // and deterministic if sessionStorage is unavailable.
    const nextUrl = new URL(window.location.href)
    nextUrl.searchParams.set('pxout', next ? '1' : '0')
    window.location.assign(nextUrl.toString())
  }

  return (
    <button
      type="button"
      data-testid="external-pixel-output-toggle"
      aria-pressed={enabled}
      aria-label={enabled ? 'Disable external pixel output' : 'Enable external pixel output'}
      title={enabled ? `External output enabled · ${url}` : `External output disabled · ${url}`}
      onClick={toggle}
      className={`flex h-8 shrink-0 items-center gap-1 rounded px-2 font-mono text-[10px] transition-colors ${
        enabled
          ? 'bg-green-500/10 text-green-400 hover:bg-green-500/20'
          : 'text-zinc-500 hover:bg-zinc-800/80 hover:text-zinc-300'
      }`}
    >
      <RadioTower size={14} aria-hidden />
      <span>OUT</span>
    </button>
  )
}
