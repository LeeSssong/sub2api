import { describe, expect, it } from 'vitest'
import { readFileSync, readdirSync } from 'node:fs'
import { resolve } from 'node:path'
import postcss from 'postcss'

describe('refresh action visibility', () => {
  it('keeps an icon and visible refresh label in every manual refresh button', () => {
    const failures: string[] = []
    let buttons = 0
    const scan = (directory: string) => {
      for (const entry of readdirSync(directory, { withFileTypes: true })) {
        const path = resolve(directory, entry.name)
        if (entry.isDirectory()) { scan(path); continue }
        if (!entry.name.endsWith('.vue')) continue
        const source = readFileSync(path, 'utf8')
        for (const match of source.matchAll(/<button\b[\s\S]*?<\/button>/g)) {
          const button = match[0]
          if (!button.includes('common.refresh')) continue
          buttons++
          if (!/<(?:Icon|svg)\b/.test(button) || !/{{[^}]*common.refresh[^}]*}}/.test(button)) failures.push(path)
        }
      }
    }
    scan(resolve(process.cwd(), 'src'))
    expect(buttons).toBeGreaterThan(0)
    expect(failures).toEqual([])
  })

  it('does not hide toolbar SVGs at any breakpoint', () => {
    const css = readFileSync(resolve(process.cwd(), 'src/styles/xingqiao-user.css'), 'utf8')
    const host = document.createElement('div')
    host.innerHTML = '<div class="user-keys-page"><div data-test="keys-actions"><button><svg data-icon="refresh"></svg>刷新</button></div></div>'
    const icon = host.querySelector('svg')!
    const hidden: string[] = []
    postcss.parse(css).walkRules(rule => {
      if (!icon.matches(rule.selector)) return
      rule.walkDecls('display', declaration => {
        if (declaration.value === 'none') hidden.push(rule.selector)
      })
    })
    expect(hidden).toEqual([])
  })
})
