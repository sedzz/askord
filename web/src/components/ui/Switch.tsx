import type { InputHTMLAttributes, ReactNode } from 'react'
import './switch.css'

type Props = Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'onChange'> & {
  checked: boolean
  onChange: (checked: boolean) => void
  children: ReactNode
}

// Interruptor con su texto; pulsar el texto también lo cambia.
export function Switch({ checked, onChange, children, className, ...props }: Props) {
  return (
    <label className={className ? `ui-switch ${className}` : 'ui-switch'}>
      <input
        type="checkbox"
        role="switch"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        {...props}
      />
      <span className="ui-switch-track" aria-hidden="true" />
      <span>{children}</span>
    </label>
  )
}
