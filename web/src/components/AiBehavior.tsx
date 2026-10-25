import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent, type MouseEvent } from 'react'
import { aiQuery, MAX_GUIDELINES, saveAiBehavior, type AiConnection } from '../lib/ai'
import { toast } from '../lib/toast'
import { Button } from './ui/Button'
import { Checkbox } from './ui/Checkbox'
import { FormField } from './ui/FormField'
import { Textarea } from './ui/Textarea'

type Props = { guildId: string; connection: AiConnection }

// A partir de cuántos caracteres o líneas las pautas se muestran recortadas.
const PREVIEW_CHARS = 280
const PREVIEW_LINES = 4

// Posición del texto de container (un nodo de texto) que hay bajo un punto de la pantalla, o
// null si ahí no hay texto suyo. Así al pulsar una palabra el cursor queda donde se pulsó.
function caretOffsetAt(x: number, y: number, container: HTMLElement): number | null {
  let node: Node | null = null
  let offset = 0
  if (document.caretPositionFromPoint) {
    const pos = document.caretPositionFromPoint(x, y)
    if (pos) [node, offset] = [pos.offsetNode, pos.offset]
  } else if (document.caretRangeFromPoint) {
    const range = document.caretRangeFromPoint(x, y)
    if (range) [node, offset] = [range.startContainer, range.startOffset]
  }
  if (!node || node.nodeType !== Node.TEXT_NODE || !container.contains(node)) return null
  return offset
}

// Interruptor general del asistente y pautas que se aplican a todos los tickets del servidor.
// Con pautas guardadas se leen en una caja (recortada si es larga, con "Ver todo"); pulsar el
// texto lo convierte en editable en el mismo sitio. Si el botón de guardar no se ve en
// pantalla mientras hay cambios, aparece uno flotante.
export function AiBehavior({ guildId, connection }: Props) {
  const queryClient = useQueryClient()
  const hasGuidelines = connection.guidelines !== ''
  const [editing, setEditing] = useState(!hasGuidelines)
  const [expanded, setExpanded] = useState(false)
  const [draft, setDraft] = useState(connection.guidelines)
  const [caret, setCaret] = useState<number | undefined>(undefined)
  const [saveVisible, setSaveVisible] = useState(true)
  const saveRef = useRef<HTMLDivElement>(null)

  const tooLong = [...draft.trim()].length > MAX_GUIDELINES
  const dirty = draft.trim() !== connection.guidelines

  // El interruptor guarda al instante, con las pautas ya guardadas (no con el borrador).
  const toggle = useMutation({
    mutationFn: (enabled: boolean) =>
      saveAiBehavior(guildId, { enabled, guidelines: connection.guidelines }),
    onSuccess: (saved) => queryClient.setQueryData(aiQuery(guildId).queryKey, saved),
  })

  const save = useMutation({
    mutationFn: () => saveAiBehavior(guildId, { enabled: connection.enabled, guidelines: draft }),
    onSuccess: (saved) => {
      queryClient.setQueryData(aiQuery(guildId).queryKey, saved)
      setDraft(saved.guidelines)
      setEditing(saved.guidelines === '')
      toast('Comportamiento guardado')
    },
  })

  // ¿Se ve el botón de guardar? Si no, aparece el flotante.
  useEffect(() => {
    const el = saveRef.current
    if (!el) return
    const observer = new IntersectionObserver(([entry]) => setSaveVisible(entry.isIntersecting))
    observer.observe(el)
    return () => observer.disconnect()
  }, [editing])

  function discard() {
    setDraft(connection.guidelines)
    setEditing(connection.guidelines === '')
    save.reset()
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (dirty && !tooLong) save.mutate()
  }

  // Empieza a editar con el cursor en la posición dada (por defecto, al final del texto).
  function startEditing(offset?: number) {
    setCaret(offset ?? connection.guidelines.length)
    setExpanded(true)
    setEditing(true)
  }

  function onReadClick(e: MouseEvent<HTMLDivElement>, textId: string) {
    const text = document.getElementById(textId)
    startEditing((text && caretOffsetAt(e.clientX, e.clientY, text)) ?? undefined)
  }

  function onReadKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      startEditing()
    }
  }

  const long =
    connection.guidelines.length > PREVIEW_CHARS ||
    connection.guidelines.split('\n').length > PREVIEW_LINES

  return (
    <section className="settings-section">
      <div className="section-head">
        <h2>Comportamiento</h2>
        <Checkbox
          checked={connection.enabled}
          disabled={toggle.isPending}
          onChange={(enabled) => toggle.mutate(enabled)}
        >
          Asistente activado
        </Checkbox>
      </div>
      {toggle.isError && <p className="error">{toggle.error.message}</p>}
      {!hasGuidelines && (
        <p>
          Cómo debe actuar el asistente. Además, cada tipo de ticket tiene sus propias instrucciones
          y decide si lo atiende la IA.
        </p>
      )}

      <form className="form ai-behavior-form" onSubmit={onSubmit}>
        <FormField
          label="Pautas generales"
          hint={
            editing
              ? 'Se aplican a todos los tickets: tono, quiénes sois, qué puede y qué no puede prometer. Admite markdown.'
              : 'Pulsa el texto para editarlo.'
          }
        >
          {({ id }) =>
            editing ? (
              <Textarea
                id={id}
                value={draft}
                onChange={(v) => {
                  setDraft(v)
                  save.reset()
                }}
                limit={MAX_GUIDELINES}
                rows={8}
                className="ai-guidelines-edit"
                autoFocusAt={hasGuidelines ? caret : undefined}
                placeholder={'Eres el asistente de soporte de…\nHabla siempre con un tono cercano.\nNunca prometas reembolsos.'}
                spellCheck
              />
            ) : (
              <>
                <div
                  id={id}
                  role="button"
                  tabIndex={0}
                  aria-label="Editar las pautas generales"
                  aria-describedby={`${id}-text`}
                  className={expanded || !long ? 'ai-guidelines editable' : 'ai-guidelines editable clamped'}
                  onClick={(e) => onReadClick(e, `${id}-text`)}
                  onKeyDown={onReadKeyDown}
                >
                  <span id={`${id}-text`}>{connection.guidelines}</span>
                </div>
                {long && (
                  <button
                    type="button"
                    className="ai-text-button"
                    onClick={() => setExpanded((e) => !e)}
                  >
                    {expanded ? 'Ver menos' : 'Ver todo'}
                  </button>
                )}
              </>
            )
          }
        </FormField>

        {editing && (
          <div className="form-actions" ref={saveRef}>
            <Button type="submit" disabled={!dirty || tooLong} loading={save.isPending}>
              {save.isPending ? 'Guardando' : 'Guardar'}
            </Button>
            {hasGuidelines && (
              <Button variant="outline" onClick={discard} disabled={save.isPending}>
                Descartar
              </Button>
            )}
            {save.isError && <span className="error">{save.error.message}</span>}
          </div>
        )}
      </form>

      {editing && dirty && !saveVisible && (
        <div className="ai-floating-save">
          <Button
            disabled={tooLong}
            loading={save.isPending}
            onClick={() => save.mutate()}
          >
            {save.isPending ? 'Guardando' : 'Guardar cambios'}
          </Button>
        </div>
      )}
    </section>
  )
}
