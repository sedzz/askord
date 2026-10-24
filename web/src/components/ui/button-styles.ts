export type ButtonVariant = 'primary' | 'outline'

// Clases del botón, para dárselas también a enlaces (<a>, <Link>) con el mismo aspecto.
export function buttonClassName(variant: ButtonVariant = 'primary', className?: string): string {
  return ['ui-button', `ui-button--${variant}`, className].filter(Boolean).join(' ')
}
