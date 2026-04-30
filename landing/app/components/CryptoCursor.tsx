'use client'

import { useEffect, useRef, useState } from 'react'

const interactiveSelector = [
  'a',
  'button',
  'summary',
  '.premium-card',
  '.showcase-card',
  '[data-cursor]',
].join(',')

export default function CryptoCursor() {
  const dotRef = useRef<HTMLDivElement>(null)
  const ringRef = useRef<HTMLDivElement>(null)
  const [enabled, setEnabled] = useState(false)

  useEffect(() => {
    const canUseCursor = window.matchMedia('(hover: hover) and (pointer: fine)')
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)')
    if (!canUseCursor.matches || reduceMotion.matches) return

    setEnabled(true)
    document.documentElement.classList.add('has-crypto-cursor')

    let raf = 0
    let hoverTarget: Element | null = null
    let magneticTarget: HTMLElement | null = null
    const target = { x: window.innerWidth / 2, y: window.innerHeight / 2 }
    const dot = { x: target.x, y: target.y }
    const ring = { x: target.x, y: target.y }

    const setHover = (next: Element | null) => {
      if (hoverTarget === next) return
      hoverTarget = next
      document.documentElement.classList.toggle('cursor-hovering', Boolean(next))
      document.documentElement.classList.toggle('cursor-card-hovering', Boolean(next?.closest('.premium-card,.showcase-card')))
    }

    const resetMagnet = () => {
      if (!magneticTarget) return
      magneticTarget.style.removeProperty('--magnet-x')
      magneticTarget.style.removeProperty('--magnet-y')
      magneticTarget = null
    }

    const onPointerMove = (event: PointerEvent) => {
      target.x = event.clientX
      target.y = event.clientY

      const element = document.elementFromPoint(event.clientX, event.clientY)
      const interactive = element?.closest(interactiveSelector) ?? null
      setHover(interactive)

      const magnetic = element?.closest('.premium-button') as HTMLElement | null
      if (magnetic) {
        magneticTarget = magnetic
        const rect = magnetic.getBoundingClientRect()
        const x = (event.clientX - rect.left - rect.width / 2) * 0.12
        const y = (event.clientY - rect.top - rect.height / 2) * 0.16
        magnetic.style.setProperty('--magnet-x', `${x.toFixed(2)}px`)
        magnetic.style.setProperty('--magnet-y', `${y.toFixed(2)}px`)
      } else {
        resetMagnet()
      }
    }

    const onPointerDown = () => {
      document.documentElement.classList.add('cursor-clicking')
      window.setTimeout(() => document.documentElement.classList.remove('cursor-clicking'), 180)
    }

    const animate = () => {
      dot.x += (target.x - dot.x) * 0.42
      dot.y += (target.y - dot.y) * 0.42
      ring.x += (target.x - ring.x) * 0.16
      ring.y += (target.y - ring.y) * 0.16

      dotRef.current?.style.setProperty('transform', `translate3d(${dot.x}px, ${dot.y}px, 0)`)
      ringRef.current?.style.setProperty('transform', `translate3d(${ring.x}px, ${ring.y}px, 0)`)

      raf = requestAnimationFrame(animate)
    }

    window.addEventListener('pointermove', onPointerMove, { passive: true })
    window.addEventListener('pointerdown', onPointerDown, { passive: true })
    raf = requestAnimationFrame(animate)

    return () => {
      cancelAnimationFrame(raf)
      window.removeEventListener('pointermove', onPointerMove)
      window.removeEventListener('pointerdown', onPointerDown)
      document.documentElement.classList.remove('has-crypto-cursor', 'cursor-hovering', 'cursor-clicking', 'cursor-card-hovering')
      resetMagnet()
    }
  }, [])

  if (!enabled) return null

  return (
    <>
      <div ref={ringRef} className="crypto-cursor-ring" aria-hidden>
        <span className="crypto-cursor-axis crypto-cursor-axis-x" />
        <span className="crypto-cursor-axis crypto-cursor-axis-y" />
      </div>
      <div ref={dotRef} className="crypto-cursor-dot" aria-hidden>
        <span />
      </div>
    </>
  )
}
