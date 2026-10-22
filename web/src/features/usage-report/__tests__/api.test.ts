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
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { downloadUsageReportCSV } from '../api'

afterEach(() => {
  document.querySelectorAll('a[download]').forEach((anchor) => anchor.remove())
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('rejects a JSON business-error blob without creating a download', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: new Blob(
      [JSON.stringify({ success: false, message: 'report unavailable' })],
      { type: 'application/json' }
    ),
  })
  const createObjectURL = vi
    .spyOn(URL, 'createObjectURL')
    .mockReturnValue('blob:usage-report')

  await expect(downloadUsageReportCSV(7, 'daily')).rejects.toThrow(
    'report unavailable'
  )
  expect(createObjectURL).not.toHaveBeenCalled()
})

test('downloads a valid CSV blob with the expected filename', async () => {
  vi.useFakeTimers()
  const blob = new Blob(['date,calls\n2026-10-22,1\n'], {
    type: 'text/csv; charset=utf-8',
  })
  vi.spyOn(api, 'get').mockResolvedValue({ data: blob })
  const createObjectURL = vi
    .spyOn(URL, 'createObjectURL')
    .mockReturnValue('blob:usage-report')
  const revokeObjectURL = vi
    .spyOn(URL, 'revokeObjectURL')
    .mockImplementation(() => undefined)
  const click = vi
    .spyOn(HTMLAnchorElement.prototype, 'click')
    .mockImplementation(() => undefined)

  await downloadUsageReportCSV(7, 'daily')

  expect(createObjectURL).toHaveBeenCalledWith(blob)
  expect(click).toHaveBeenCalledTimes(1)
  const anchor = document.querySelector<HTMLAnchorElement>(
    'a[download="usage_report_daily_7d.csv"]'
  )
  expect(anchor).not.toBeNull()
  await vi.advanceTimersByTimeAsync(300)
  expect(revokeObjectURL).toHaveBeenCalledWith('blob:usage-report')
  expect(anchor).not.toBeInTheDocument()
})
