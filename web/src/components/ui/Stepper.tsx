import './stepper.css'

type Props = {
  // Paso actual, desde 0.
  current: number
  total: number
}

// Indicador de progreso por segmentos, con "Paso X de N".
export function Stepper({ current, total }: Props) {
  const label = `Paso ${current + 1} de ${total}`
  return (
    <div
      className="ui-stepper"
      role="progressbar"
      aria-label={label}
      aria-valuemin={1}
      aria-valuemax={total}
      aria-valuenow={current + 1}
    >
      <div className="ui-stepper-bars">
        {Array.from({ length: total }, (_, i) => (
          <span key={i} className={i <= current ? 'ui-stepper-bar done' : 'ui-stepper-bar'} />
        ))}
      </div>
      <span className="ui-stepper-label">{label}</span>
    </div>
  )
}
