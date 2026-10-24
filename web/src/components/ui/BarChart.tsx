import { useState, type MouseEvent } from 'react'
import './chart.css'

type Datum = { label: string; value: number }

type Props = {
  data: Datum[]
  // Texto del globo al pasar por una barra: "3 tickets".
  valueLabel: (value: number) => string
  // Resumen para lectores de pantalla.
  ariaLabel: string
}

// Separación horizontal entre el ratón y el globo.
const GAP = 28

// Cifra par y redonda por encima del máximo, para que las marcas (0, mitad, máximo) sean enteras.
function niceMax(max: number): number {
  const top = Math.max(2, max)
  const unit = 10 ** Math.floor(Math.log10(top))
  let rounded = Math.ceil(top / unit) * unit
  if (rounded % 2) rounded += unit
  return rounded
}

// Gráfica de barras sencilla, hecha con HTML y CSS (sin librerías).
export function BarChart({ data, valueLabel, ariaLabel }: Props) {
  const [active, setActive] = useState<number | null>(null)
  // Posición del ratón dentro de la gráfica; el globo la sigue.
  const [pointer, setPointer] = useState({ x: 0, y: 0, width: 1, height: 1 })
  const max = niceMax(Math.max(0, ...data.map((d) => d.value)))
  const middle = data[Math.floor((data.length - 1) / 2)]
  const hovered = active === null ? undefined : data[active]

  function onMouseMove(e: MouseEvent<HTMLDivElement>) {
    const rect = e.currentTarget.getBoundingClientRect()
    setPointer({
      x: e.clientX - rect.left,
      y: e.clientY - rect.top,
      width: rect.width,
      height: rect.height,
    })
  }

  return (
    <div className="ui-chart" role="img" aria-label={ariaLabel}>
      <div className="ui-chart-y" aria-hidden="true">
        <span>{max}</span>
        <span>{max / 2}</span>
        <span>0</span>
      </div>

      <div className="ui-chart-plot" onMouseMove={onMouseMove} onMouseLeave={() => setActive(null)}>
        <div className="ui-chart-grid" aria-hidden="true">
          <span />
          <span />
          <span />
        </div>
        <div className="ui-chart-bars">
          {data.map((d, i) => (
            <div key={d.label + i} className="ui-chart-col" onMouseEnter={() => setActive(i)}>
              <div
                className={active === i ? 'ui-chart-bar active' : 'ui-chart-bar'}
                style={{
                  height: `${(d.value / max) * 100}%`,
                  minHeight: d.value > 0 ? 3 : 0,
                }}
              />
            </div>
          ))}
        </div>
        {hovered && active !== null && (
          <div
            className="ui-chart-tooltip"
            // A la altura del ratón y separado de él; pasada la mitad de la gráfica, a su izquierda.
            style={{
              left: pointer.x > pointer.width * 0.6 ? pointer.x - GAP : pointer.x + GAP,
              top: Math.min(Math.max(pointer.y, 24), pointer.height - 24),
              transform:
                pointer.x > pointer.width * 0.6 ? 'translate(-100%, -50%)' : 'translateY(-50%)',
            }}
          >
            <strong>{valueLabel(hovered.value)}</strong>
            <span>{hovered.label}</span>
          </div>
        )}
      </div>

      {data.length > 0 && (
        <div className="ui-chart-x" aria-hidden="true">
          <span>{data[0].label}</span>
          <span>{middle.label}</span>
          <span>{data[data.length - 1].label}</span>
        </div>
      )}
    </div>
  )
}
