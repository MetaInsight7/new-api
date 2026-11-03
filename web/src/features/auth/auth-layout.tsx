/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { TokenDaziBrandLogo } from '@/components/brand/token-dazi-brand-logo'

import './auth.css'

/* ─── Mascot Constants ─── */

const clamp = (value: number, min: number, max: number) =>
  Math.min(Math.max(value, min), max)

interface CreatureDef {
  key: string
  bodyClass: string
  eyesClass: string
  speedClass: string
  eyeType: 'white' | 'solid'
  baseLeft: number
  baseTop: number
  eyeSize: number
  pupilSize?: number
  maxDistance: number
  mouth?: boolean
}

const CREATURES: CreatureDef[] = [
  {
    key: 'violet',
    bodyClass: 'auth-creature-violet',
    eyesClass: 'auth-eyes-violet',
    speedClass: 'auth-eyes-slow',
    eyeType: 'white',
    baseLeft: 45,
    baseTop: 40,
    eyeSize: 18,
    pupilSize: 7,
    maxDistance: 5,
  },
  {
    key: 'ink',
    bodyClass: 'auth-creature-ink',
    eyesClass: 'auth-eyes-ink',
    speedClass: 'auth-eyes-slow',
    eyeType: 'white',
    baseLeft: 26,
    baseTop: 32,
    eyeSize: 16,
    pupilSize: 6,
    maxDistance: 4,
  },
  {
    key: 'coral',
    bodyClass: 'auth-creature-coral',
    eyesClass: 'auth-eyes-coral',
    speedClass: 'auth-eyes-fast',
    eyeType: 'solid',
    baseLeft: 82,
    baseTop: 90,
    eyeSize: 12,
    maxDistance: 5,
  },
  {
    key: 'sun',
    bodyClass: 'auth-creature-sun',
    eyesClass: 'auth-eyes-sun',
    speedClass: 'auth-eyes-fast',
    eyeType: 'solid',
    baseLeft: 52,
    baseTop: 40,
    eyeSize: 12,
    maxDistance: 5,
    mouth: true,
  },
]

const EYE_COUNT = 2
const AUTH_POINTER_STORAGE_KEY = 'new-api-auth-pointer'

interface Pointer {
  x: number
  y: number
  animateOnAuthEnter?: boolean
  synthetic?: boolean
}

type AuthWindow = Window & {
  __newApiAuthPointer?: Pointer
}

const RIGHT_ENTRY_POINTER: Pointer = {
  x: Number.POSITIVE_INFINITY,
  y: Number.NaN,
  animateOnAuthEnter: true,
  synthetic: true,
}

let lastAuthPointer: Pointer | null = null

function getStoredAuthPointer(): Pointer | null {
  if (typeof window === 'undefined') return null

  const authWindow = window as AuthWindow
  if (authWindow.__newApiAuthPointer || lastAuthPointer) {
    return authWindow.__newApiAuthPointer || lastAuthPointer
  }

  try {
    const saved = window.sessionStorage.getItem(AUTH_POINTER_STORAGE_KEY)
    if (!saved) return null
    const parsedPointer: unknown = JSON.parse(saved)
    if (typeof parsedPointer !== 'object' || parsedPointer === null) return null
    const pointer = parsedPointer as Record<string, unknown>
    if (
      typeof pointer.x !== 'number' ||
      typeof pointer.y !== 'number' ||
      !Number.isFinite(pointer.x) ||
      !Number.isFinite(pointer.y)
    ) {
      return null
    }
    return { x: pointer.x, y: pointer.y, animateOnAuthEnter: true }
  } catch {
    return null
  }
}

function setStoredAuthPointer(
  pointer: Pointer,
  options: Partial<Pointer> = {}
) {
  const next = { ...pointer, ...options }
  lastAuthPointer = next
  if (typeof window !== 'undefined') {
    const authWindow = window as AuthWindow
    authWindow.__newApiAuthPointer = next
    if (!next.synthetic && Number.isFinite(next.x) && Number.isFinite(next.y)) {
      try {
        window.sessionStorage.setItem(
          AUTH_POINTER_STORAGE_KEY,
          JSON.stringify({ x: next.x, y: next.y })
        )
      } catch {}
    }
  }
}

/* ─── Mascot Pose ─── */

interface EyeStyle {
  transform: string
}

interface CreaturePose {
  bodyStyle: { transform: string }
  eyesStyle: { left: string; top: string }
  eyeStyles: EyeStyle[]
}

interface MascotPose {
  creatures: Record<string, CreaturePose>
  mouthStyle: { left: string; top: string }
}

function createMascotPose(
  opts: {
    faceX?: number
    faceY?: number
    bodySkew?: number
    lookRight?: boolean
  } = {}
): MascotPose {
  const { faceX = 0, faceY = 0, bodySkew = 0, lookRight = false } = opts
  return {
    creatures: Object.fromEntries(
      CREATURES.map((creature) => [
        creature.key,
        {
          bodyStyle: { transform: `skewX(${bodySkew}deg)` },
          eyesStyle: {
            left: `${creature.baseLeft + faceX}px`,
            top: `${creature.baseTop + faceY}px`,
          },
          eyeStyles: Array.from({ length: EYE_COUNT }, () => ({
            transform: `translate(${lookRight ? creature.maxDistance : 0}px, 0px)`,
          })),
        },
      ])
    ),
    mouthStyle: { left: `${40 + faceX}px`, top: `${88 + faceY}px` },
  }
}

function cloneMascotPose(pose: MascotPose): MascotPose {
  return {
    creatures: Object.fromEntries(
      CREATURES.map((creature) => {
        const creaturePose = pose.creatures[creature.key]
        return [
          creature.key,
          {
            bodyStyle: { ...creaturePose.bodyStyle },
            eyesStyle: { ...creaturePose.eyesStyle },
            eyeStyles: creaturePose.eyeStyles.map((eyeStyle) => ({
              ...eyeStyle,
            })),
          },
        ]
      })
    ),
    mouthStyle: { ...pose.mouthStyle },
  }
}

const ENTRY_MASCOT_POSE = createMascotPose()
let lastMascotPose = ENTRY_MASCOT_POSE

/* ─── Hooks ─── */

function useRandomBlink() {
  const [isBlinking, setIsBlinking] = useState(false)

  useEffect(() => {
    let timeoutId: ReturnType<typeof setTimeout>
    const scheduleBlink = () => {
      timeoutId = setTimeout(
        () => {
          setIsBlinking(true)
          timeoutId = setTimeout(() => {
            setIsBlinking(false)
            scheduleBlink()
          }, 150)
        },
        Math.random() * 4000 + 3000
      )
    }
    scheduleBlink()
    return () => clearTimeout(timeoutId)
  }, [])

  return isBlinking
}

/* ─── Motion helpers ─── */

function getFaceMotion(element: Element, pointer: Pointer) {
  const rect = element.getBoundingClientRect()
  const centerX = rect.left + rect.width / 2
  const faceAnchorY = rect.top + rect.height / 3
  const pointerX = Number.isFinite(pointer.x) ? pointer.x : centerX + 300
  const pointerY = Number.isFinite(pointer.y) ? pointer.y : faceAnchorY
  const deltaX = pointerX - centerX
  return {
    faceX: clamp(deltaX / 20, -15, 15),
    faceY: clamp((pointerY - faceAnchorY) / 30, -10, 10),
    bodySkew: clamp(-deltaX / 120, -6, 6),
  }
}

function getEyeOffset(element: Element, pointer: Pointer, maxDistance: number) {
  const rect = element.getBoundingClientRect()
  const centerX = rect.left + rect.width / 2
  const centerY = rect.top + rect.height / 2
  const pointerX = Number.isFinite(pointer.x) ? pointer.x : centerX + 300
  const pointerY = Number.isFinite(pointer.y) ? pointer.y : centerY
  const deltaX = pointerX - centerX
  const deltaY = pointerY - centerY
  const distance = Math.min(Math.sqrt(deltaX ** 2 + deltaY ** 2), maxDistance)
  const angle = Math.atan2(deltaY, deltaX)
  return { x: Math.cos(angle) * distance, y: Math.sin(angle) * distance }
}

function getEyeTargets(eyes: Element, creature: CreatureDef) {
  if (creature.eyeType === 'white') {
    return [...eyes.querySelectorAll<HTMLElement>('.auth-white-eye')]
      .map((eye) => ({
        anchor: eye,
        target: eye.querySelector<HTMLElement>('.auth-pupil'),
      }))
      .filter(
        (
          eyeTarget
        ): eyeTarget is { anchor: HTMLElement; target: HTMLElement } =>
          Boolean(eyeTarget.target)
      )
  }
  return [...eyes.querySelectorAll<HTMLElement>('.auth-black-eye')].map(
    (eye) => ({
      anchor: eye,
      target: eye,
    })
  )
}

function applyMascotMotion(root: HTMLElement, pointer: Pointer) {
  const nextPose = cloneMascotPose(lastMascotPose)
  let sunMotion: ReturnType<typeof getFaceMotion> | null = null

  for (const creature of CREATURES) {
    const body = root.querySelector(`.${creature.bodyClass}`) as HTMLElement
    const eyes = root.querySelector(`.${creature.eyesClass}`) as HTMLElement
    if (!body || !eyes) return

    const motion = getFaceMotion(body, pointer)
    const creaturePose = nextPose.creatures[creature.key]
    creaturePose.bodyStyle = {
      transform: `skewX(${motion.bodySkew || 0}deg)`,
    }
    creaturePose.eyesStyle = {
      left: `${creature.baseLeft + motion.faceX}px`,
      top: `${creature.baseTop + motion.faceY}px`,
    }
    Object.assign(body.style, creaturePose.bodyStyle)
    Object.assign(eyes.style, creaturePose.eyesStyle)

    getEyeTargets(eyes, creature).forEach(({ anchor, target }, index) => {
      const maxDist = Number(anchor.dataset.maxDistance || creature.maxDistance)
      const offset = getEyeOffset(anchor, pointer, maxDist)
      const style = { transform: `translate(${offset.x}px, ${offset.y}px)` }
      target.style.transform = style.transform
      creaturePose.eyeStyles[index] = style
    })

    if (creature.mouth) sunMotion = motion
  }

  if (sunMotion) {
    const mouth = root.querySelector('.auth-mouth') as HTMLElement
    if (!mouth) return
    nextPose.mouthStyle = {
      left: `${40 + sunMotion.faceX}px`,
      top: `${88 + sunMotion.faceY}px`,
    }
    Object.assign(mouth.style, nextPose.mouthStyle)
  }

  lastMascotPose = nextPose
}

/* ─── Mascot Components ─── */

function MascotEye({
  creature,
  isBlinking,
  motionStyle,
}: {
  creature: CreatureDef
  isBlinking: boolean
  motionStyle: EyeStyle
}) {
  if (creature.eyeType === 'white') {
    return (
      <span
        className='auth-white-eye'
        data-max-distance={creature.maxDistance}
        style={{
          width: creature.eyeSize,
          height: isBlinking ? 2 : creature.eyeSize,
        }}
      >
        {!isBlinking && (
          <span
            className='auth-pupil'
            style={{
              width: creature.pupilSize,
              height: creature.pupilSize,
              ...motionStyle,
            }}
          />
        )}
      </span>
    )
  }
  return (
    <span
      className='auth-black-eye'
      data-max-distance={creature.maxDistance}
      style={{
        width: creature.eyeSize,
        height: creature.eyeSize,
        ...motionStyle,
      }}
    />
  )
}

function MascotCreature({
  creature,
  pose,
  isBlinking,
}: {
  creature: CreatureDef
  pose: MascotPose
  isBlinking: boolean
}) {
  const creaturePose = pose.creatures[creature.key]
  return (
    <div
      className={`auth-creature ${creature.bodyClass}`}
      style={creaturePose.bodyStyle}
    >
      <div
        className={`auth-eyes ${creature.eyesClass} ${creature.speedClass}`}
        style={creaturePose.eyesStyle}
      >
        {Array.from({ length: EYE_COUNT }, (_, eyeIndex) => (
          <MascotEye
            key={eyeIndex}
            creature={creature}
            isBlinking={isBlinking}
            motionStyle={creaturePose.eyeStyles[eyeIndex]}
          />
        ))}
      </div>
      {creature.mouth && (
        <span className='auth-mouth' style={pose.mouthStyle} />
      )}
    </div>
  )
}

function AuthMascot() {
  const mascotRef = useRef<HTMLDivElement>(null)
  const frameRef = useRef<number | null>(null)
  const pointerRef = useRef<Pointer | null>(null)

  const initialPointer = getStoredAuthPointer() || RIGHT_ENTRY_POINTER
  lastAuthPointer = initialPointer
  const shouldAnimateOnEnter = Boolean(initialPointer?.animateOnAuthEnter)
  const pose = shouldAnimateOnEnter ? ENTRY_MASCOT_POSE : lastMascotPose

  const blinking: Record<string, boolean> = {
    violet: useRandomBlink(),
    ink: useRandomBlink(),
  }

  useLayoutEffect(() => {
    const root = mascotRef.current
    let firstFrame: number | null = null
    let secondFrame: number | null = null

    if (!window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      const storedPointer = getStoredAuthPointer() || RIGHT_ENTRY_POINTER
      const shouldAnimate = Boolean(storedPointer?.animateOnAuthEnter)
      pointerRef.current = storedPointer

      if (!shouldAnimate) {
        root?.classList.add('auth-mascot-no-transition')
      }

      firstFrame = requestAnimationFrame(() => {
        if (!shouldAnimate && storedPointer && root) {
          setStoredAuthPointer(storedPointer, { animateOnAuthEnter: false })
          applyMascotMotion(root, storedPointer)
        }
        secondFrame = requestAnimationFrame(() => {
          if (shouldAnimate && storedPointer && root) {
            setStoredAuthPointer(storedPointer, { animateOnAuthEnter: false })
            applyMascotMotion(root, storedPointer)
          }
          root?.classList.remove('auth-mascot-no-transition')
        })
      })
    }

    return () => {
      if (firstFrame) cancelAnimationFrame(firstFrame)
      if (secondFrame) cancelAnimationFrame(secondFrame)
    }
  }, [])

  useEffect(() => {
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)')

    const applyMotion = () => {
      frameRef.current = null
      const root = mascotRef.current
      const pointer = pointerRef.current
      if (!root || !pointer || reducedMotion.matches) return
      applyMascotMotion(root, pointer)
    }

    const handleMouseMove = (event: MouseEvent) => {
      const pointer = { x: event.clientX, y: event.clientY }
      setStoredAuthPointer(pointer)
      pointerRef.current = pointer
      if (frameRef.current || reducedMotion.matches) return
      frameRef.current = requestAnimationFrame(applyMotion)
    }

    window.addEventListener('mousemove', handleMouseMove, { passive: true })
    return () => {
      window.removeEventListener('mousemove', handleMouseMove)
      if (frameRef.current) cancelAnimationFrame(frameRef.current)
    }
  }, [])

  return (
    <div ref={mascotRef} className='auth-mascot' aria-hidden='true'>
      {CREATURES.map((creature) => (
        <MascotCreature
          key={creature.key}
          creature={creature}
          pose={pose}
          isBlinking={Boolean(blinking[creature.key])}
        />
      ))}
    </div>
  )
}

/* ─── AuthLayout ─── */

type AuthLayoutProps = {
  children: React.ReactNode
  compact?: boolean
}

export function AuthLayout({ children, compact }: AuthLayoutProps) {
  const { t } = useTranslation()
  const [navigationOpen, setNavigationOpen] = useState(false)
  const menuButtonRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    const mobileBreakpoint = window.matchMedia('(max-width: 760px)')
    const closeNavigation = () => setNavigationOpen(false)
    mobileBreakpoint.addEventListener('change', closeNavigation)
    return () => mobileBreakpoint.removeEventListener('change', closeNavigation)
  }, [])

  useEffect(() => {
    if (!navigationOpen) return

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setNavigationOpen(false)
      menuButtonRef.current?.focus()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [navigationOpen])

  const closeNavigation = () => setNavigationOpen(false)

  return (
    <div className={`auth-page${compact ? ' auth-page-compact' : ''}`}>
      <header className='auth-navbar'>
        <nav className='auth-navbar-inner' aria-label={t('Main navigation')}>
          <a
            href='https://tokendazi.com/'
            className='auth-navbar-brand'
            aria-label={t('Token Dazi home')}
          >
            <TokenDaziBrandLogo alt='' />
          </a>
          <div
            id='auth-navbar-links'
            className={`auth-navbar-links${navigationOpen ? ' open' : ''}`}
          >
            <a href='https://tokendazi.com/' onClick={closeNavigation}>
              {t('Home')}
            </a>
            <a href='https://wiki.tokendazi.com/cn' onClick={closeNavigation}>
              {t('Quick start')}
            </a>
            <a href='https://tokendazi.com/models' onClick={closeNavigation}>
              {t('Models')}
            </a>
            <a href='https://tokendazi.com/rankings' onClick={closeNavigation}>
              {t('Rankings')}
            </a>
          </div>
          <div className='auth-navbar-actions'>
            <a href='/sign-in'>{t('Login')}</a>
            <a
              className='auth-navbar-action-primary'
              href='/sign-up?redirect=/workspace'
            >
              {t('Free registration')}
            </a>
          </div>
          <button
            ref={menuButtonRef}
            className='auth-navbar-menu'
            type='button'
            aria-label={t(
              navigationOpen ? 'Close navigation' : 'Open navigation'
            )}
            aria-controls='auth-navbar-links'
            aria-expanded={navigationOpen}
            onClick={() => setNavigationOpen((open) => !open)}
          >
            <span aria-hidden='true'>☰</span>
          </button>
        </nav>
      </header>

      <div className='auth-body'>
        <section className='auth-visual-panel' aria-hidden='true'>
          <AuthMascot />
        </section>

        <section className='auth-form-panel'>
          <div className='auth-form-stack'>
            <div className='auth-form-shell'>{children}</div>
          </div>
        </section>
      </div>
    </div>
  )
}
