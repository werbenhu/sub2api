import type { Group } from '@/types'
import { getPlatformSpec, listPlatformIds } from '@/constants/platformCatalog'

export type MagpieImportGroup = Pick<Group, 'platform' | 'platforms' | 'claude_code_only'>

export interface MagpieImportInput {
  baseUrl: string
  providerName: string
  apiKey: string
  groups: readonly MagpieImportGroup[]
}

type MagpieApi = 'chat' | 'responses' | 'anthropic' | 'gemini'

function gatewayRoot(baseUrl: string): string {
  const url = new URL(baseUrl)
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
    throw new Error('Invalid gateway URL')
  }
  url.pathname = url.pathname.replace(/\/+$/, '').replace(/\/(v1|v1beta)$/, '')
  return url.toString().replace(/\/+$/, '')
}

/** Official format: https://usemagpie.ai/docs/import. Keep credentials in the fragment. */
export function buildMagpieImportUrl(input: MagpieImportInput): string {
  const root = gatewayRoot(input.baseUrl)
  const endpoints: Partial<Record<MagpieApi, string>> = {}
  const groups: readonly MagpieImportGroup[] = input.groups.length
    ? input.groups
    : [{ platform: 'anthropic', claude_code_only: false }]
  const platforms = new Set(groups.flatMap(group =>
    group.platform === 'composite'
      ? group.platforms?.length ? group.platforms : listPlatformIds()
      : [group.platform]
  ))
  // A shared endpoint lets the gateway choose among every bound group/platform.
  // Dedicated Antigravity paths are useful only when it is the sole platform.
  const antigravityRoot = platforms.size === 1 && platforms.has('antigravity')
    ? `${root}/antigravity`
    : root

  for (const group of groups) {
    const selected = group.platform === 'composite'
      ? group.platforms?.length ? group.platforms : listPlatformIds()
      : [group.platform]
    for (const platform of selected) {
      // System One is a separate API that Magpie cannot import.
      if (platform === 'typesafe') continue
      if (group.claude_code_only) {
        endpoints.anthropic = platform === 'antigravity' ? antigravityRoot : root
        continue
      }
      switch (getPlatformSpec(platform)?.gateway) {
        case 'openai':
          endpoints.chat = `${root}/v1`
          endpoints.responses = `${root}/v1`
          break
        case 'gemini':
          endpoints.gemini = `${root}/v1beta`
          break
        case 'antigravity':
          endpoints.anthropic = antigravityRoot
          endpoints.gemini = `${antigravityRoot}/v1beta`
          break
        default:
          endpoints.anthropic = root
      }
    }
  }

  if (!Object.keys(endpoints).length) throw new Error('No supported Magpie API')
  const name = Array.from(input.providerName.trim() || 'sub2api').slice(0, 80).join('')
  const params = new URLSearchParams({ name, ...endpoints, key: input.apiKey })
  return `https://usemagpie.ai/import#${params.toString()}`
}
