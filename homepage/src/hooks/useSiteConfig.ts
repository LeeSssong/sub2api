import { useEffect, useState } from 'react'
import { DEFAULT_SITE_CONFIG, loadSiteConfig, type SiteConfig } from '../domain/siteConfig'
import { updateFavicon } from '../domain/branding'

function initialConfig(): SiteConfig {
  return {
    ...DEFAULT_SITE_CONFIG,
    apiOrigin: window.location.origin,
    support: { ...DEFAULT_SITE_CONFIG.support },
    thirdPartyReports: [],
  }
}

export function useSiteConfig() {
  const [config, setConfig] = useState<SiteConfig>(initialConfig)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let active = true
    loadSiteConfig(fetch, window.location.origin).then((next) => {
      if (active) {
        setConfig(next)
        setLoaded(true)
      }
    })
    return () => { active = false }
  }, [])

  useEffect(() => {
    if (loaded) updateFavicon(config.siteLogo)
  }, [config.siteLogo, loaded])

  return config
}
