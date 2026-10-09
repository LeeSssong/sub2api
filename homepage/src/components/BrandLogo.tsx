import { DEFAULT_SITE_LOGO } from '../domain/branding'

interface BrandLogoProps {
  src?: string
  width?: number
  height?: number
}

export function BrandLogo({ src = DEFAULT_SITE_LOGO, width, height }: BrandLogoProps) {
  return <img src={src} alt="" width={width} height={height} onError={(event) => {
    if (event.currentTarget.getAttribute('src') !== DEFAULT_SITE_LOGO) {
      event.currentTarget.setAttribute('src', DEFAULT_SITE_LOGO)
    }
  }} />
}
