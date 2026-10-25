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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../select'

let triggerTop = 240

function rect(left: number, top: number, width: number, height: number) {
  return {
    bottom: top + height,
    height,
    left,
    right: left + width,
    top,
    width,
    x: left,
    y: top,
    toJSON: () => undefined,
  } as DOMRect
}

function ScrollableSelect() {
  return (
    <div
      data-testid='scroll-container'
      style={{ height: 160, overflow: 'auto' }}
    >
      <div style={{ height: 480, paddingTop: 200 }}>
        <Select defaultValue='default'>
          <SelectTrigger aria-label='Group'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='default'>Default</SelectItem>
            <SelectItem value='vip'>VIP</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}

beforeEach(() => {
  triggerTop = 240
  Object.defineProperty(document.documentElement, 'clientWidth', {
    configurable: true,
    value: 1440,
  })
  Object.defineProperty(document.documentElement, 'clientHeight', {
    configurable: true,
    value: 900,
  })
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
    function (this: HTMLElement) {
      if (this.dataset.slot === 'select-trigger') {
        return rect(120, triggerTop, 180, 32)
      }
      if (
        this.dataset.slot === 'select-content' ||
        this.querySelector?.('[data-slot="select-content"]')
      ) {
        return rect(120, triggerTop + 36, 180, 80)
      }
      return rect(0, 0, 0, 0)
    }
  )
  vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockImplementation(
    function (this: HTMLElement) {
      return this.dataset.slot === 'select-trigger' ? 180 : 180
    }
  )
  vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockImplementation(
    function (this: HTMLElement) {
      return this.dataset.slot === 'select-trigger' ? 32 : 80
    }
  )
})

describe('Select popup positioning', () => {
  it('moves the open popup with its trigger when an ancestor scrolls', async () => {
    render(<ScrollableSelect />)
    const user = userEvent.setup()
    await user.click(screen.getByRole('combobox', { name: 'Group' }))

    const listbox = await screen.findByRole('listbox')
    const popup = listbox.closest<HTMLElement>('[data-slot="select-content"]')
    const positioner = popup?.parentElement
    if (!positioner) throw new Error('Expected Select positioner')

    expect(popup).toHaveAttribute('data-align-trigger', 'false')
    await waitFor(() => expect(positioner.style.cssText).not.toBe(''))
    const before = positioner.style.cssText
    triggerTop -= 40
    fireEvent.scroll(screen.getByTestId('scroll-container'))

    await waitFor(() => expect(positioner.style.cssText).not.toBe(before))
  })
})
