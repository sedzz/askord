import type { ButtonHTMLAttributes } from 'react'
import { buttonClassName, type ButtonVariant } from './button-styles'
import { Spinner } from './Spinner'
import './button.css'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant
  // Muestra un spinner delante del texto y bloquea el botón. El texto es el que se pase
  // como hijo: el que lo usa decide si cambia ("Guardando…") o se queda igual.
  loading?: boolean
}

export function Button({
  variant = 'primary',
  className,
  type = 'button',
  loading = false,
  disabled,
  children,
  ...props
}: Props) {
  return (
    <button
      type={type}
      className={buttonClassName(variant, className)}
      {...props}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
    >
      {loading && <Spinner />}
      {children}
    </button>
  )
}
