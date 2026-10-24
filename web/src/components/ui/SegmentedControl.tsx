import './segmented-control.css'

type Option<T extends string | number> = { value: T; label: string }

type Props<T extends string | number> = {
  value: T
  onChange: (value: T) => void
  options: Option<T>[]
  label: string
}

// Grupo de opciones excluyentes, en una sola barra (por ejemplo, el rango de una gráfica).
export function SegmentedControl<T extends string | number>({
  value,
  onChange,
  options,
  label,
}: Props<T>) {
  return (
    <div className="ui-segmented" role="group" aria-label={label}>
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          className={o.value === value ? 'ui-segment active' : 'ui-segment'}
          aria-pressed={o.value === value}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}
