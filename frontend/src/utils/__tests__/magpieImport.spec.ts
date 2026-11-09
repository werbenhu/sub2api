import { describe, expect, it } from 'vitest'
import type { GroupPlatform } from '@/types'
import { buildMagpieImportUrl, type MagpieImportGroup } from '../magpieImport'

const group = (platform: GroupPlatform, platforms?: string[]): MagpieImportGroup => ({
  platform, platforms, claude_code_only: false
})
const baseInput = {
  baseUrl: 'https://api.example.com',
  providerName: '测试站点 - My Key',
  apiKey: 'sk-test+&=#中文',
  groups: [group('anthropic')]
}
const paramsFor = (groups: MagpieImportGroup[], baseUrl = baseInput.baseUrl) =>
  new URLSearchParams(new URL(buildMagpieImportUrl({ ...baseInput, baseUrl, groups })).hash.slice(1))

describe('Magpie provider import', () => {
  it('uses the official web link and keeps the API key exclusively in the fragment', () => {
    const url = new URL(buildMagpieImportUrl(baseInput))
    expect(`${url.origin}${url.pathname}`).toBe('https://usemagpie.ai/import')
    expect(url.search).toBe('')
    const params = new URLSearchParams(url.hash.slice(1))
    expect(params.get('key')).toBe(baseInput.apiKey)
    expect(params.get('name')).toBe(baseInput.providerName)
    expect(params.has('models')).toBe(false) // Discover the key's current model catalog.
  })

  it.each(['', '/', '/v1', '/v1/', '/v1beta/'])('normalizes %s without duplicating API paths', suffix => {
    const params = paramsFor([group('composite', ['anthropic', 'openai', 'gemini'])], `https://api.example.com/proxy${suffix}`)
    expect(params.get('anthropic')).toBe('https://api.example.com/proxy')
    expect(params.get('chat')).toBe('https://api.example.com/proxy/v1')
    expect(params.get('responses')).toBe('https://api.example.com/proxy/v1')
    expect(params.get('gemini')).toBe('https://api.example.com/proxy/v1beta')
  })

  it.each(['openai', 'grok', 'kimi', 'zhipu', 'deepseek', 'minimax'])('imports %s through the OpenAI gateway', platform => {
    const params = paramsFor([group(platform)])
    expect(params.get('chat')).toBe('https://api.example.com/v1')
    expect(params.get('responses')).toBe('https://api.example.com/v1')
    expect(params.has('gemini')).toBe(false)
  })

  it('uses only the native Gemini API for a Gemini group', () => {
    const params = paramsFor([group('gemini')])
    expect(params.get('gemini')).toBe('https://api.example.com/v1beta')
    expect(params.has('anthropic')).toBe(false)
    expect(params.has('chat')).toBe(false)
  })

  it('imports both Antigravity APIs without requiring client selection', () => {
    const params = paramsFor([group('antigravity')])
    expect(params.get('anthropic')).toBe('https://api.example.com/antigravity')
    expect(params.get('gemini')).toBe('https://api.example.com/antigravity/v1beta')
  })

  it('combines bound groups through shared endpoints', () => {
    const params = paramsFor([group('anthropic'), group('openai'), group('antigravity'), group('gemini')])
    expect(params.get('anthropic')).toBe('https://api.example.com')
    expect(params.get('gemini')).toBe('https://api.example.com/v1beta')
    expect(params.get('responses')).toBe('https://api.example.com/v1')
  })

  it('includes only the selected composite platforms', () => {
    const params = paramsFor([group('composite', ['anthropic', 'openai'])])
    expect(params.has('anthropic')).toBe(true)
    expect(params.has('chat')).toBe(true)
    expect(params.has('gemini')).toBe(false)
  })

  it('supports unrestricted legacy composite groups', () => {
    const params = paramsFor([group('composite', [])])
    for (const api of ['anthropic', 'chat', 'responses', 'gemini']) expect(params.has(api)).toBe(true)
  })

  it('keeps Claude-only groups on the Messages API', () => {
    const params = paramsFor([{ ...group('openai'), claude_code_only: true }])
    expect(params.get('anthropic')).toBe('https://api.example.com')
    expect(params.has('chat')).toBe(false)
    expect(params.has('responses')).toBe(false)
  })

  it('does not invent an OpenAI endpoint for System One', () => {
    expect(() => paramsFor([group('typesafe')])).toThrow('No supported Magpie API')
    expect(paramsFor([group('typesafe'), group('openai')]).has('chat')).toBe(true)
  })

  it('falls back to the default Anthropic gateway for an unbound key', () => {
    expect(paramsFor([]).get('anthropic')).toBe('https://api.example.com')
  })

  it('allows local HTTP gateways', () => {
    expect(paramsFor([group('openai')], 'http://localhost:8080/v1/').get('responses')).toBe('http://localhost:8080/v1')
  })

  it.each(['javascript:alert(1)', 'https://user:pass@api.example.com', 'https://api.example.com?token=x', 'https://api.example.com#x'])('rejects invalid gateway URLs: %s', baseUrl => {
    expect(() => buildMagpieImportUrl({ ...baseInput, baseUrl })).toThrow('Invalid gateway URL')
  })

  it('limits names to the documented 80 characters without breaking Unicode', () => {
    const url = new URL(buildMagpieImportUrl({ ...baseInput, providerName: '🦜'.repeat(90) }))
    const params = new URLSearchParams(url.hash.slice(1))
    expect(Array.from(params.get('name')!)).toHaveLength(80)
  })
})
