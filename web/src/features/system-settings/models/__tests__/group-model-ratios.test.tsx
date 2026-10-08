import {
  render,
  screen,
  waitFor,
  within,
  fireEvent,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { GroupModelRatioEditor } from '../group-model-ratio-editor'
import {
  createGroupModelRatioSchema,
  formatBillingModelName,
} from '../lib/group-model-ratio'

function Fixture(props: {
  initial?: string
  groupRatio?: string
  onChange: (value: string) => void
  onValidityChange?: (valid: boolean) => void
}) {
  const [value, setValue] = useState(props.initial ?? '{}')
  return (
    <GroupModelRatioEditor
      value={value}
      groupRatio={props.groupRatio ?? '{"default":1,"vip":0.8}'}
      onChange={(next) => {
        setValue(next)
        props.onChange(next)
      }}
      onValidityChange={props.onValidityChange ?? (() => {})}
    />
  )
}

describe('model-specific ratio editor', () => {
  it('leaves unedited stored model keys unchanged even when their normalized keys overlap', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    const onValidityChange = vi.fn()
    render(
      <Fixture
        initial='{"vip":{"gpt-4o-gizmo-legacy":0.3,"gpt-4o-gizmo-*":0.7}}'
        onChange={onChange}
        onValidityChange={onValidityChange}
      />
    )
    const name = screen.getByRole('textbox', {
      name: 'Billing model key for vip / gpt-4o-gizmo-legacy',
    })
    await user.click(name)
    await user.tab()
    expect(name).toHaveAttribute('aria-invalid', 'false')
    expect(onChange).not.toHaveBeenCalled()
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true))
  })

  it('preserves arbitrary stored keys instead of dropping JSON keys that overlap Object.prototype', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <Fixture
        initial='{"__proto__":{"legacy":0.9},"retired":{"__proto__":0.7}}'
        onChange={onChange}
      />
    )
    await user.click(
      screen.getByRole('button', { name: 'Add model ratio for default' })
    )
    expect(JSON.parse(onChange.mock.lastCall?.[0] ?? 'null')).toEqual(
      JSON.parse(
        '{"__proto__":{"legacy":0.9},"retired":{"__proto__":0.7},"default":{"model-1":1}}'
      )
    )
  })

  it('adds a model to a pricing group and normalizes only the edited model key', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <Fixture initial='{"retired":{"legacy-key":0.7}}' onChange={onChange} />
    )
    await user.click(
      screen.getByRole('button', { name: 'Add model ratio for vip' })
    )
    const name = screen.getByRole('textbox', {
      name: 'Billing model key for vip / model-1',
    })
    await user.clear(name)
    await user.type(name, 'gemini-2.5-flash-lite-thinking-128')
    await user.tab()
    expect(
      screen.getByRole('textbox', {
        name: 'Billing model key for vip / gemini-2.5-flash-lite-thinking-*',
      })
    ).toHaveValue('gemini-2.5-flash-lite-thinking-*')
    expect(JSON.parse(onChange.mock.lastCall?.[0] ?? 'null')).toEqual({
      retired: { 'legacy-key': 0.7 },
      vip: { 'gemini-2.5-flash-lite-thinking-*': 1 },
    })
  })

  it('removes the last override without removing other groups or models', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <Fixture
        initial='{"vip":{"gpt-5":0.3},"retired":{"legacy-key":0.7}}'
        onChange={onChange}
      />
    )
    await user.click(
      screen.getByRole('button', { name: 'Remove model ratio for vip / gpt-5' })
    )
    expect(JSON.parse(onChange.mock.lastCall?.[0] ?? 'null')).toEqual({
      retired: { 'legacy-key': 0.7 },
    })
    expect(
      screen.getByRole('button', { name: 'Model ratios for vip' })
    ).toBeVisible()
    expect(
      screen.queryByRole('spinbutton', { name: 'Ratio for vip / gpt-5' })
    ).not.toBeInTheDocument()
  })

  it('adds an unknown group without creating a pricing-group configuration', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Fixture onChange={onChange} groupRatio='{}' />)
    expect(screen.getByText('No model-specific ratios')).toBeVisible()
    await user.type(
      screen.getByRole('textbox', { name: 'Group for model-specific ratios' }),
      ' archived '
    )
    await user.click(
      screen.getByRole('button', { name: 'Add model ratio group' })
    )
    expect(JSON.parse(onChange.mock.lastCall?.[0] ?? 'null')).toEqual({
      archived: { 'model-1': 1 },
    })
    expect(
      screen.getByRole('button', { name: 'Model ratios for archived' })
    ).toHaveAttribute('aria-expanded', 'true')
  })

  it('rejects a duplicate normalized model key visibly instead of overwriting its ratio', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    const onValidityChange = vi.fn()
    render(
      <Fixture
        initial='{"vip":{"gpt-5":0.3,"gpt-4o-gizmo-*":0.7}}'
        onChange={onChange}
        onValidityChange={onValidityChange}
      />
    )
    const name = screen.getByRole('textbox', {
      name: 'Billing model key for vip / gpt-5',
    })
    await user.clear(name)
    await user.type(name, 'gpt-4o-gizmo-custom')
    await user.tab()
    expect(name).toHaveAttribute('aria-invalid', 'true')
    expect(
      screen.getByText('This model already has a ratio in this group.')
    ).toBeVisible()
    expect(onChange).not.toHaveBeenCalled()
    await waitFor(() =>
      expect(onValidityChange).toHaveBeenLastCalledWith(false)
    )
    await user.clear(name)
    await user.type(name, 'gpt-5')
    await user.tab()
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true))
  })

  it('keeps an empty ratio invalid across collapse and never converts it to free', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    const onValidityChange = vi.fn()
    render(
      <Fixture
        initial='{"vip":{"gpt-5":0.3}}'
        onChange={onChange}
        onValidityChange={onValidityChange}
      />
    )
    const ratio = screen.getByRole('spinbutton', {
      name: 'Ratio for vip / gpt-5',
    })
    await user.clear(ratio)
    expect(ratio).toHaveAttribute('aria-invalid', 'true')
    expect(onChange).not.toHaveBeenCalled()
    const toggle = screen.getByRole('button', { name: 'Model ratios for vip' })
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await waitFor(() =>
      expect(onValidityChange).toHaveBeenLastCalledWith(false)
    )
    await user.click(toggle)
    expect(ratio).toHaveValue(null)
    await user.type(ratio, '0')
    expect(JSON.parse(onChange.mock.lastCall?.[0] ?? 'null')).toEqual({
      vip: { 'gpt-5': 0 },
    })
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true))
  })

  it('rejects negative and overflowing numeric drafts without changing the persisted value', async () => {
    const onChange = vi.fn()
    render(<Fixture initial='{"vip":{"gpt-5":0.3}}' onChange={onChange} />)
    const ratio = screen.getByRole('spinbutton', {
      name: 'Ratio for vip / gpt-5',
    })
    fireEvent.change(ratio, { target: { value: '-0.1' } })
    expect(ratio).toHaveAttribute('aria-invalid', 'true')
    fireEvent.change(ratio, { target: { value: '1e999' } })
    expect(ratio).toHaveAttribute('aria-invalid', 'true')
    expect(onChange).not.toHaveBeenCalled()
  })

  it('preserves invalid stored configuration and directs administrators to JSON repair', async () => {
    const onChange = vi.fn()
    const onValidityChange = vi.fn()
    render(
      <Fixture
        initial='{"vip":{"gpt-5":"0.3"}}'
        onChange={onChange}
        onValidityChange={onValidityChange}
      />
    )
    const editor = screen.getByRole('region', {
      name: 'Model-specific group ratios',
    })
    expect(
      within(editor).getByText('Invalid model-specific ratios')
    ).toBeVisible()
    expect(
      within(editor).getByRole('textbox', {
        name: 'Group for model-specific ratios',
      })
    ).toBeDisabled()
    expect(onChange).not.toHaveBeenCalled()
    await waitFor(() =>
      expect(onValidityChange).toHaveBeenLastCalledWith(false)
    )
  })
})

describe('model-specific ratio JSON validation', () => {
  const schema = createGroupModelRatioSchema((key) => key)

  it('allows explicit zero and preserves unknown group and model keys', () => {
    const value = '{"retired":{"unknown-model":0},"vip":{"gpt-5":0.123456}}'
    expect(schema.parse(value)).toBe(value)
    expect(schema.parse('{}')).toBe('{}')
  })

  it.each([
    '',
    '{',
    'null',
    '[]',
    '{"vip":[]}',
    '{"vip":{"gpt-5":"0.3"}}',
    '{"vip":{"gpt-5":-1}}',
    '{"vip":{"gpt-5":1e999}}',
    '{" ":{"gpt-5":1}}',
    '{"vip":{" ":1}}',
  ])('rejects invalid or unsafe model ratio configuration %s', (value) => {
    expect(schema.safeParse(value).success).toBe(false)
  })

  it.each([
    ['gemini-2.5-flash-lite-thinking-128', 'gemini-2.5-flash-lite-thinking-*'],
    ['gemini-2.5-flash-thinking-0', 'gemini-2.5-flash-thinking-*'],
    ['gemini-2.5-pro-thinking-1024', 'gemini-2.5-pro-thinking-*'],
    ['gpt-4-gizmo-custom', 'gpt-4-gizmo-*'],
    ['gpt-4o-gizmo-custom', 'gpt-4o-gizmo-*'],
    [' claude-sonnet-4-5 ', 'claude-sonnet-4-5'],
    ['gemini-2.5-flash-lite', 'gemini-2.5-flash-lite'],
  ])('normalizes %s to the backend billing key %s', (name, expected) => {
    expect(formatBillingModelName(name)).toBe(expected)
  })
})
