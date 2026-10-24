import { MISSING_PERMISSIONS, type Option } from '../lib/tickets'

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

// Selector de opciones con ayuda, botón para crear la opción y aviso de permisos.
export function Select({
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
  return (
    <div className="field">
      <label htmlFor={label}>{label}</label>
      <small>{help}</small>
      <div className="field-row">
        <select
          id={label}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          disabled={!options.length}
        >
          <option value="">{options.length ? 'Selecciona…' : 'No hay ninguno'}</option>
          {options.map((o) => (
            <option key={o.id} value={o.id}>
              {optionPrefix}
              {o.name}
            </option>
          ))}
        </select>
        <button type="button" onClick={onCreate} disabled={creating}>
          {createLabel}
        </button>
      </div>
      {action && (
        <div className="field-row">
          <button type="button" onClick={action.onClick} disabled={action.disabled}>
            {action.label}
          </button>
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
