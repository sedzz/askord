import type { CSSProperties } from 'react'
import './skeleton.css'

type Props = {
  width?: number | string
  height?: number | string
  // Radio de las esquinas; "50%" para círculos.
  radius?: number | string
  className?: string
}

// Bloque gris con un brillo que lo recorre: ocupa el sitio de lo que aún está cargando.
export function Skeleton({ width, height, radius, className }: Props) {
  const style: CSSProperties = { width, height, borderRadius: radius }
  return (
    <div
      className={className ? `ui-skeleton ${className}` : 'ui-skeleton'}
      style={style}
      aria-hidden="true"
    />
  )
}
