import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../AppLayout.vue'),
  'utf8',
)

describe('AppLayout regular user shell', () => {
  it('keeps the admin header and provides the fixed user header', () => {
    expect(source).toContain('<AppHeader v-if="isAdmin" />')
    expect(source).toContain('<UserHeader v-else />')
  })

  it('restores the native locale switcher on the user mobile header', () => {
    expect(source).toContain("import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'")
    expect(source).toContain('data-testid="user-mobile-locale"')
    expect(source).toContain('<LocaleSwitcher />')
  })
})
