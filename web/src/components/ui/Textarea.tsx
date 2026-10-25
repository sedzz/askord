import { useLayoutEffect, useRef, type TextareaHTMLAttributes } from 'react'
import './textarea.css'

type Props = Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'value' | 'onChange'> & {
  value: string
  onChange: (value: string) => void
  // Máximo de caracteres: se muestra un contador y se pone en rojo al pasarlo. No bloquea la
  // escritura; quien lo usa decide si deja guardar.
  limit?: number
  // Al montarse, enfoca el cuadro con el cursor en esa posición del texto.
  autoFocusAt?: number
}

// Cuadro de texto de varias líneas con el estilo de la app. Crece con lo que se escribe, así
// que no tiene barra de desplazamiento; rows es la altura mínima.
export function Textarea({ value, onChange, limit, autoFocusAt, className, rows = 6, ...props }: Props) {
  const ref = useRef<HTMLTextAreaElement>(null)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const fit = () => {
      // Para medir hay que encoger el cuadro, y con él la página: si estás desplazado hacia
      // abajo el navegador recoloca el scroll y no lo restaura. El contenedor guarda su altura
      // mientras tanto, para que la página no se encoja.
      const wrap = el.parentElement
      if (wrap) wrap.style.minHeight = `${wrap.offsetHeight}px`
      el.style.height = 'auto'
      // scrollHeight no incluye el borde: se suma para que no quede una barra por un píxel.
      el.style.height = `${el.scrollHeight + (el.offsetHeight - el.clientHeight)}px`
      if (wrap) wrap.style.minHeight = ''
    }
    fit()
    // Al cambiar el ancho de la ventana el texto se reparte en otras líneas.
    window.addEventListener('resize', fit)
    return () => window.removeEventListener('resize', fit)
  }, [value])

  // Solo al montarse: más tarde el cursor es del usuario.
  useLayoutEffect(() => {
    const el = ref.current
    if (!el || autoFocusAt === undefined) return
    el.focus()
    el.setSelectionRange(autoFocusAt, autoFocusAt)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Se cuentan caracteres, no unidades UTF-16 (un emoji vale uno), como hace el servidor.
  const count = [...value.trim()].length
  return (
    <div className="ui-textarea-wrap">
      <textarea
        ref={ref}
        className={className ? `ui-textarea ${className}` : 'ui-textarea'}
        value={value}
        rows={rows}
        onChange={(e) => onChange(e.target.value)}
        {...props}
      />
      {limit !== undefined && (
        <span className={count > limit ? 'ui-textarea-count over' : 'ui-textarea-count'}>
          {count} / {limit}
        </span>
      )}
    </div>
  )
}
