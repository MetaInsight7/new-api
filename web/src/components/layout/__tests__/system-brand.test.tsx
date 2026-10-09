import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { SidebarProvider } from '@/components/ui/sidebar'

import { SystemBrand } from '../components/system-brand'

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({
    status: { system_name: 'Configured Console', version: 'v-test' },
  }),
}))

vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({ logo: '/configured-logo.svg' }),
}))

describe('console system branding', () => {
  it('uses the final Overlay brand and opens the separate website from the top bar', () => {
    render(<SystemBrand variant='inline' />)

    const homeLink = screen.getByRole('link', { name: 'Go to home' })
    expect(homeLink).toHaveAttribute('href', 'https://tokendazi.com/')
    expect(homeLink).toHaveAttribute('target', '_blank')
    expect(homeLink).toHaveAttribute('rel', 'noopener noreferrer')
    expect(screen.getByRole('img', { name: 'Logo' })).toHaveAttribute(
      'src',
      '/token-dazi-logo.svg'
    )
    expect(screen.getByText('Token 搭子')).toBeInTheDocument()
  })

  it('preserves the configurable system identity, logo and version in the sidebar variant', () => {
    render(
      <SidebarProvider>
        <SystemBrand variant='sidebar' />
      </SidebarProvider>
    )

    expect(screen.getByText('Configured Console')).toBeInTheDocument()
    expect(screen.getByText('v-test')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Logo' })).toHaveAttribute(
      'src',
      '/configured-logo.svg'
    )
    expect(screen.queryByText('Token 搭子')).toBeNull()
  })
})
