import { describe, expect, it, vi } from 'vitest'
import { resolveDocumentTitle, resolveRouteDocumentTitle, resolveRouteMetaKeys } from '@/router/title'

vi.mock('@/i18n', () => ({
  i18n: { global: { t: (key: string) => key } },
}))

describe('resolveDocumentTitle', () => {
  it('formats route and site titles', () => {
    expect(resolveDocumentTitle('Usage Records', 'My Site')).toBe('Usage Records - My Site')
    expect(resolveDocumentTitle(undefined, 'My Site')).toBe('My Site')
    expect(resolveDocumentTitle(undefined, '')).toBe('Sub2API')
  })
})

describe('resolveRouteDocumentTitle', () => {
  it('uses the custom menu label after it loads', () => {
    const route = {
      name: 'CustomPage',
      params: { id: 'scheduler' },
      meta: { title: 'Custom Page' },
    }

    expect(resolveRouteDocumentTitle(route, 'EzouAPI')).toBe('Custom Page - EzouAPI')
    expect(resolveRouteDocumentTitle(route, 'EzouAPI', [{
      id: 'scheduler',
      label: '账号调度器',
      icon_svg: '',
      url: 'https://example.com',
      visibility: 'admin',
      sort_order: 0,
    }])).toBe('账号调度器 - EzouAPI')
  })
})

describe('resolveRouteMetaKeys', () => {
  it('uses route metadata directly', () => {
    expect(resolveRouteMetaKeys({
      meta: { titleKey: 'userSubscriptions.title', descriptionKey: 'userSubscriptions.description' },
    })).toEqual({
      titleKey: 'userSubscriptions.title',
      descriptionKey: 'userSubscriptions.description',
    })
  })
})
