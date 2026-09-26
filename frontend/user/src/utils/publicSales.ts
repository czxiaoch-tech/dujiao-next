export type PublicSaleState = 'native' | 'live' | 'coming_soon'

export interface PublicSaleConfig {
  state: PublicSaleState
  checkoutUrl?: string
}

const PUBLIC_SALES: Record<string, PublicSaleConfig> = {
  'chatgpt-plus': {
    state: 'live',
    checkoutUrl: 'https://catfk.com/item/sj2q9t',
  },
  'chatgpt-pro-5x': { state: 'coming_soon' },
  'chatgpt-pro-20x': { state: 'coming_soon' },
  'chatgpt-ios-pro-20x': { state: 'coming_soon' },
}

export const getPublicSaleConfig = (slug?: string | null): PublicSaleConfig => {
  const key = String(slug || '').trim().toLowerCase()
  return PUBLIC_SALES[key] || { state: 'native' }
}

export const isExternalSaleLive = (slug?: string | null) => {
  const config = getPublicSaleConfig(slug)
  return config.state === 'live' && !!config.checkoutUrl
}
