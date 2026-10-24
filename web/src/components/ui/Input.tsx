import type { InputHTMLAttributes } from 'react'
import './input.css'

type Props = InputHTMLAttributes<HTMLInputElement>

// Campo de texto (y de fecha, número…) con el estilo de la app.
export function Input({ className, ...props }: Props) {
  return <input className={className ? `ui-input ${className}` : 'ui-input'} {...props} />
}
