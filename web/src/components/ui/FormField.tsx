import { useId, type ReactNode } from 'react'
import './form-field.css'

type Props = {
  label: string
  hint?: string
  // Con un selector propio (que no es un input), el nombre se da con aria-labelledby: un
  // <label for> reenviaría los clics al botón y lo abriría al pulsar el texto.
  custom?: boolean
  // Recibe los ids con los que enlazar el control con su etiqueta.
  children: (ids: { id: string; labelId: string }) => ReactNode
}

// Etiqueta, control y ayuda de un campo de formulario.
export function FormField({ label, hint, custom = false, children }: Props) {
  const id = useId()
  const labelId = `${id}-label`
  return (
    <div className="ui-field">
      {custom ? (
        <span id={labelId} className="ui-field-label">
          {label}
        </span>
      ) : (
        <label id={labelId} htmlFor={id} className="ui-field-label">
          {label}
        </label>
      )}
      {children({ id, labelId })}
      {hint && <small>{hint}</small>}
    </div>
  )
}
