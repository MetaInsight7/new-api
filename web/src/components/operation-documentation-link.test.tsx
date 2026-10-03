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
*/
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import { OperationDocumentationLink } from './operation-documentation-link'

afterEach(() => {
  document.body.replaceChildren()
})

describe('OperationDocumentationLink', () => {
  it('opens the configured documentation safely in a new tab', () => {
    render(<OperationDocumentationLink />)

    const documentationButton = screen.getByRole('button', {
      name: 'Operation documentation',
    })
    expect(documentationButton).toHaveAttribute(
      'href',
      'https://docs.flatkey.ai/'
    )
    expect(documentationButton).toHaveAttribute('target', '_blank')
    expect(documentationButton).toHaveAttribute('rel', 'noreferrer noopener')
  })
})
