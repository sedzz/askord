import { useEffect, useId, useRef, type ReactNode } from 'react'
import './dialog.css'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  // Con false no se puede cerrar con Esc, la × ni pulsando fuera: solo lo cierra el padre.
  dismissible?: boolean
  children: ReactNode
}

// Diálogo modal sobre el <dialog> nativo: foco atrapado, Esc para cerrar y fondo bloqueado.
export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  dismissible = true,
  children,
}: Props) {
  const ref = useRef<HTMLDialogElement>(null)
  const titleId = useId()

  useEffect(() => {
    const dialog = ref.current
    if (!dialog) return
    if (open && !dialog.open) dialog.showModal()
    if (!open && dialog.open) dialog.close()
  }, [open])

  return (
    <dialog
      ref={ref}
      className="ui-dialog"
      aria-labelledby={titleId}
      // Esc: se cancela el cierre si el diálogo no es descartable.
      onCancel={(e) => {
        if (!dismissible) e.preventDefault()
      }}
      // Esc y dialog.close() disparan "close": el estado del padre se mantiene al día.
      // Algunos navegadores cierran con un segundo Esc aunque se cancele el primero:
      // si sigue abierto por el padre y no es descartable, se vuelve a abrir.
      onClose={() => {
        if (!dismissible && open) ref.current?.showModal()
        else onOpenChange(false)
      }}
      // El relleno va en el contenedor interior, así que un clic sobre el propio
      // <dialog> solo puede venir del fondo oscuro.
      onClick={(e) => {
        if (dismissible && e.target === e.currentTarget) onOpenChange(false)
      }}
    >
      <div className="ui-dialog-body">
        <header className="ui-dialog-header">
          <h2 id={titleId}>{title}</h2>
          {dismissible && (
            <button
              type="button"
              className="ui-dialog-close"
              aria-label="Cerrar"
              onClick={() => onOpenChange(false)}
            >
              ×
            </button>
          )}
        </header>
        {description && <p className="ui-dialog-description">{description}</p>}
        {open && children}
      </div>
    </dialog>
  )
}
