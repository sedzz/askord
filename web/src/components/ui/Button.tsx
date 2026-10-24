import type { ButtonHTMLAttributes } from 'react'
import { buttonClassName, type ButtonVariant } from './button-styles'
import './button.css'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant
}

export function Button({ variant = 'primary', className, type = 'button', ...props }: Props) {
  return <button type={type} className={buttonClassName(variant, className)} {...props} />
}
