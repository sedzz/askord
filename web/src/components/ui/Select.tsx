import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import './select.css'

export type SelectOption = { value: string; label: string }

type Props = {
  value: string
  onChange: (value: string) => void
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  id?: string
  className?: string
  // Id del elemento que da nombre al selector (en lugar de un <label for>, que lo abriría al pulsarlo).
  labelledBy?: string
}

type Position = { left: number; width: number; maxHeight: number; top?: number; bottom?: number }

const MAX_LIST_HEIGHT = 240
const GAP = 4
const ROW_HEIGHT = 36

// Selector propio (patrón "select-only combobox"): botón + lista de opciones con teclado.
// La lista usa posición fija para no quedar recortada dentro de diálogos con scroll.
export function Select({
  value,
  onChange,
  options,
  placeholder = 'Selecciona…',
  disabled,
  id,
  className,
  labelledBy,
}: Props) {
  const autoId = useId()
  const triggerId = id ?? `${autoId}-trigger`
  const listId = `${autoId}-list`
  const triggerRef = useRef<HTMLButtonElement>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const [position, setPosition] = useState<Position | null>(null)

  const selectedIndex = options.findIndex((o) => o.value === value)
  const selected = selectedIndex >= 0 ? options[selectedIndex] : undefined

  // initialActive: opción resaltada al abrir. Con el ratón no hay ninguna (el fondo solo
  // aparece al pasar por encima); con el teclado, la seleccionada.
  function openList(initialActive: number) {
    const trigger = triggerRef.current
    if (!trigger || disabled || options.length === 0) return

    // Se abre hacia arriba si abajo no cabe y arriba hay más sitio.
    const rect = trigger.getBoundingClientRect()
    const below = window.innerHeight - rect.bottom - GAP - 8
    const above = rect.top - GAP - 8
    const wanted = Math.min(MAX_LIST_HEIGHT, options.length * ROW_HEIGHT + 8)
    const flip = below < wanted && above > below

    setPosition({
      left: rect.left,
      width: rect.width,
      maxHeight: Math.min(MAX_LIST_HEIGHT, flip ? above : below),
      ...(flip ? { bottom: window.innerHeight - rect.top + GAP } : { top: rect.bottom + GAP }),
    })
    setActive(initialActive)
    setOpen(true)
  }

  const openFromKeyboard = () => openList(selectedIndex >= 0 ? selectedIndex : 0)

  function choose(index: number) {
    const option = options[index]
    if (option) onChange(option.value)
    setOpen(false)
  }

  // Cierra al pulsar fuera, al redimensionar o al hacer scroll fuera de la lista.
  useEffect(() => {
    if (!open) return
    const inside = (target: EventTarget | null) =>
      target instanceof Node &&
      (triggerRef.current?.contains(target) || listRef.current?.contains(target))
    const onPointerDown = (e: PointerEvent) => {
      if (!inside(e.target)) setOpen(false)
    }
    const onScroll = (e: Event) => {
      if (!inside(e.target)) setOpen(false)
    }
    const close = () => setOpen(false)

    document.addEventListener('pointerdown', onPointerDown)
    window.addEventListener('scroll', onScroll, true)
    window.addEventListener('resize', close)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      window.removeEventListener('scroll', onScroll, true)
      window.removeEventListener('resize', close)
    }
  }, [open])

  // Mueve solo el scroll de la lista (no el de la página) para dejar visible una opción.
  function scrollToOption(index: number) {
    const list = listRef.current
    const item = list?.children[index] as HTMLElement | undefined
    if (!list || !item) return
    if (item.offsetTop < list.scrollTop) {
      list.scrollTop = item.offsetTop
    } else if (item.offsetTop + item.offsetHeight > list.scrollTop + list.clientHeight) {
      list.scrollTop = item.offsetTop + item.offsetHeight - list.clientHeight
    }
  }

  // Al abrir, deja a la vista la opción seleccionada.
  useEffect(() => {
    if (open) scrollToOption(selectedIndex)
  }, [open, selectedIndex])

  // Con el teclado, sigue a la opción activa.
  useEffect(() => {
    if (open && active >= 0) scrollToOption(active)
  }, [open, active])

  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>) {
    if (disabled) return
    const last = options.length - 1
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        if (open) setActive((i) => Math.min(i + 1, last))
        else openFromKeyboard()
        break
      case 'ArrowUp':
        e.preventDefault()
        if (open) setActive((i) => Math.max(i - 1, 0))
        else openFromKeyboard()
        break
      case 'Home':
        if (open) {
          e.preventDefault()
          setActive(0)
        }
        break
      case 'End':
        if (open) {
          e.preventDefault()
          setActive(last)
        }
        break
      case 'Enter':
      case ' ':
        e.preventDefault()
        if (open) choose(active)
        else openFromKeyboard()
        break
      case 'Escape':
        // Se cancela para que un diálogo contenedor no se cierre a la vez.
        if (open) {
          e.preventDefault()
          setOpen(false)
        }
        break
      case 'Tab':
        setOpen(false)
        break
    }
  }

  return (
    <div className={className ? `ui-select ${className}` : 'ui-select'}>
      <button
        ref={triggerRef}
        id={triggerId}
        type="button"
        role="combobox"
        className="ui-select-trigger"
        aria-haspopup="listbox"
        aria-labelledby={labelledBy}
        aria-expanded={open}
        aria-controls={listId}
        aria-activedescendant={open && active >= 0 ? `${listId}-${active}` : undefined}
        disabled={disabled}
        onClick={() => (open ? setOpen(false) : openList(-1))}
        onKeyDown={onKeyDown}
      >
        <span className={selected ? 'ui-select-value' : 'ui-select-value placeholder'}>
          {selected?.label ?? placeholder}
        </span>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <path d="m6 9 6 6 6-6" />
        </svg>
      </button>

      {open && position && (
        <ul
          ref={listRef}
          id={listId}
          role="listbox"
          className="ui-select-list"
          style={position}
          // Al salir con el ratón, ninguna opción queda resaltada.
          onPointerLeave={() => setActive(-1)}
        >
          {options.map((option, i) => (
            <li
              key={option.value}
              id={`${listId}-${i}`}
              role="option"
              aria-selected={option.value === value}
              className={i === active ? 'ui-select-option active' : 'ui-select-option'}
              onPointerEnter={() => setActive(i)}
              // Evita que el clic le quite el foco al botón.
              onPointerDown={(e) => e.preventDefault()}
              onClick={() => choose(i)}
            >
              <span>{option.label}</span>
              {option.value === value && (
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <path d="M20 6 9 17l-5-5" />
                </svg>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
