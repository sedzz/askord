import { useId } from 'react'
import { MISSING_PERMISSIONS, type Option } from '../lib/tickets'
import { Button } from './ui/Button'
import { Select } from './ui/Select'

type Props = {
  label: string
  help: string
  value: string
  options: Option[]
  onChange: (value: string) => void
  optionPrefix?: string
  createLabel: string
  onCreate: () => void
  creating: boolean
  createError: Error | null
  missingPermissionsHint: string
  // Acción principal opcional junto al selector (por ejemplo, publicar el panel).
  action?: { label: string; onClick: () => void; disabled: boolean }
}

// Campo con etiqueta, ayuda, selector, botón para crear la opción y aviso de permisos.
export function SelectField({
  label,
  help,
  value,
  options,
  onChange,
  optionPrefix = '',
  createLabel,
  onCreate,
  creating,
  createError,
  missingPermissionsHint,
  action,
}: Props) {
  const id = useId()

  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <small>{help}</small>
      <div className="field-row">
        <Select
          id={id}
          value={value}
          onChange={onChange}
          options={options.map((o) => ({ value: o.id, label: optionPrefix + o.name }))}
          placeholder={options.length ? 'Selecciona…' : 'No hay ninguno'}
          disabled={!options.length}
        />
        <Button variant="outline" onClick={onCreate} disabled={creating}>
          {createLabel}
        </Button>
      </div>
      {action && (
        <div className="field-row">
          <Button onClick={action.onClick} disabled={action.disabled}>
            {action.label}
          </Button>
        </div>
      )}
      {createError && (
        <small className="error">
          {createError.message === MISSING_PERMISSIONS
            ? missingPermissionsHint
            : 'No se pudo crear. Inténtalo de nuevo.'}
        </small>
      )}
    </div>
  )
}
