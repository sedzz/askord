import type { InputHTMLAttributes, ReactNode } from 'react'
import './checkbox.css'

type Props = Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'onChange'> & {
  checked: boolean
  onChange: (checked: boolean) => void
  children: ReactNode
}

// Casilla con su texto; pulsar el texto también la marca.
export function Checkbox({ checked, onChange, children, className, ...props }: Props) {
  return (
    <label className={className ? `ui-checkbox ${className}` : 'ui-checkbox'}>
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        {...props}
      />
      <span>{children}</span>
    </label>
  )
}
