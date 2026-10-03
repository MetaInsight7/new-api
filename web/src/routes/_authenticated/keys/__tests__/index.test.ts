import { describe, expect, test } from 'vitest'

import { clearCreateDialogSearch, validateApiKeySearch } from '../index'

describe('API key route create dialog search', () => {
  test('keeps a valid group only for a create deep link', () => {
    expect(
      validateApiKeySearch({ open: 'create', group: 'premium', page: 2 })
    ).toMatchObject({ open: 'create', group: 'premium', page: 2 })
    expect(validateApiKeySearch({ group: 'premium' })).not.toHaveProperty(
      'group'
    )
  })

  test('clears create parameters without changing other table search state', () => {
    expect(
      clearCreateDialogSearch({
        open: 'create',
        group: 'premium',
        page: 3,
        filter: 'demo',
      })
    ).toEqual({
      open: undefined,
      group: undefined,
      page: 3,
      filter: 'demo',
    })
  })
})
