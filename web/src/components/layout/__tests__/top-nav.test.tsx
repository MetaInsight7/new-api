import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { TopNav } from '../components/top-nav'
import type { TopNavLink } from '../types'

const websiteLinks: TopNavLink[] = [
  { title: 'Home', href: 'https://tokendazi.com/', external: true },
  { title: 'Models', href: 'https://tokendazi.com/models', external: true },
]

describe('top website navigation', () => {
  it('uses safe new-tab website links in the desktop navigation', () => {
    render(<TopNav links={websiteLinks} />)

    const modelsLink = within(screen.getByRole('navigation')).getByRole(
      'link',
      { name: 'Models' }
    )
    expect(modelsLink).toHaveAttribute('href', 'https://tokendazi.com/models')
    expect(modelsLink).toHaveAttribute('target', '_blank')
    expect(modelsLink).toHaveAttribute('rel', 'noopener noreferrer')
  })

  it('opens the mobile navigation by keyboard and returns focus when dismissed', async () => {
    const user = userEvent.setup()
    render(<TopNav links={websiteLinks} />)

    const trigger = screen.getByRole('button', { name: 'Open menu' })
    trigger.focus()
    await user.keyboard('{Enter}')

    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('menuitem', { name: 'Models' })).toHaveAttribute(
      'href',
      'https://tokendazi.com/models'
    )

    await user.keyboard('{Escape}')

    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(trigger).toHaveFocus()
  })

  it('marks a disabled external desktop entry as unavailable and removes it from the tab order', () => {
    render(<TopNav links={[{ ...websiteLinks[0], disabled: true }]} />)

    const disabledLink = screen.getByRole('link', { name: 'Home' })
    expect(disabledLink).toHaveAttribute('aria-disabled', 'true')
    expect(disabledLink).toHaveAttribute('tabindex', '-1')
  })

  it('does not activate a disabled external entry in the mobile dropdown', async () => {
    const user = userEvent.setup()
    render(<TopNav links={[{ ...websiteLinks[0], disabled: true }]} />)

    await user.click(screen.getByRole('button', { name: 'Open menu' }))
    expect(screen.getByRole('menuitem', { name: 'Home' })).toHaveAttribute(
      'aria-disabled',
      'true'
    )
  })

  it('does not show an empty navigation dropdown when no links exist', () => {
    render(<TopNav links={[]} />)

    expect(screen.queryByRole('button', { name: 'Open menu' })).toBeNull()
    expect(screen.queryByRole('navigation')).toBeNull()
  })
})
