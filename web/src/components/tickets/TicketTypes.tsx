import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { restrictToParentElement, restrictToVerticalAxis } from '@dnd-kit/modifiers'
import { CSS } from '@dnd-kit/utilities'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState, type FormEvent, type ReactNode } from 'react'
import { aiQuery, MAX_AI_ESCALATION, MAX_AI_INSTRUCTIONS } from '../../lib/ai'
import { useSetup } from '../../lib/ticketSetupContext'
import { toast } from '../../lib/toast'
import {
  createTicketType,
  deleteTicketType,
  MAX_QUESTION_LABEL,
  MAX_QUESTIONS,
  MAX_TICKET_TYPES,
  reorderTicketTypes,
  ticketTypesQuery,
  updateTicketType,
  type TicketType,
  type Question,
  type TicketTypeInput,
} from '../../lib/ticketTypes'
import './ticket-types.css'
import { Button } from '../ui/Button'
import { Checkbox } from '../ui/Checkbox'
import { Dialog } from '../ui/Dialog'
import { FormField } from '../ui/FormField'
import { Input } from '../ui/Input'
import { Select } from '../ui/Select'
import { Skeleton } from '../ui/Skeleton'
import { Textarea } from '../ui/Textarea'

// Los tipos de la lista puestos en el orden de ids (los que ya no existan se descartan).
const inOrder = (list: TicketType[], ids: number[]) =>
  ids.map((id) => list.find((t) => t.id === id)).filter((t) => t !== undefined)

// Tipos de ticket del servidor: lista, orden, alta, edición y borrado.
export function TicketTypes({ guildId }: { guildId: string }) {
  const queryClient = useQueryClient()
  const queryKey = ticketTypesQuery(guildId).queryKey
  const { data: fetched, isError } = useQuery(ticketTypesQuery(guildId))
  // Orden recién soltado, mientras se guarda. El estado de React se actualiza en el mismo
  // lote que el fin del arrastre; la caché de la consulta avisa un instante después y, en ese
  // hueco, la fila soltada se veía un momento en el sitio viejo.
  const [pendingOrder, setPendingOrder] = useState<number[] | null>(null)
  const types = fetched && pendingOrder ? inOrder(fetched, pendingOrder) : fetched
  const { config } = useSetup()

  // null: ningún diálogo abierto; 'new': crear; un tipo: editarlo o borrarlo.
  const [editing, setEditing] = useState<TicketType | 'new' | null>(null)
  const [removing, setRemoving] = useState<TicketType | null>(null)

  const reorder = useMutation({
    mutationFn: (ids: number[]) => reorderTicketTypes(guildId, ids),
    onSuccess: (_, ids) =>
      queryClient.setQueryData<TicketType[]>(queryKey, (old) => old && inOrder(old, ids)),
    // Si falla, se vuelve a pedir el orden real.
    onError: () => {
      toast('No se pudo cambiar el orden', 'error')
      void queryClient.invalidateQueries({ queryKey })
    },
    onSettled: () => setPendingOrder(null),
  })

  // El arrastre empieza tras mover 6 px, para que un clic normal no lo active.
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  function handleDragEnd({ active, over }: DragEndEvent) {
    if (!types || !over || active.id === over.id) return
    const ids = types.map((t) => t.id)
    const from = ids.indexOf(Number(active.id))
    const to = ids.indexOf(Number(over.id))
    if (from < 0 || to < 0) return
    const next = arrayMove(ids, from, to)
    setPendingOrder(next)
    reorder.mutate(next)
  }

  const nameOf = (list: { id: string; name: string }[], id: string) =>
    list.find((o) => o.id === id)?.name

  return (
    <section className="ticket-types">
      <div className="section-head">
        <div>
          <h2>Tipos de ticket</h2>
          <p className="card-sub">
            Lo que elegirán tus miembros al abrir un ticket. Cada tipo va a su propia categoría y
            equipo. El panel publicado en Discord se actualiza solo con cada cambio.
          </p>
        </div>
        <Button
          onClick={() => setEditing('new')}
          disabled={!types || types.length >= MAX_TICKET_TYPES}
        >
          Añadir tipo
        </Button>
      </div>

      {!types ? (
        isError ? (
          <p className="error">No se pudieron cargar los tipos.</p>
        ) : (
          <div className="skeleton-stack">
            <Skeleton height={56} />
            <Skeleton height={56} />
          </div>
        )
      ) : (
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          modifiers={[restrictToVerticalAxis, restrictToParentElement]}
          onDragEnd={handleDragEnd}
        >
          <SortableContext items={types.map((t) => t.id)} strategy={verticalListSortingStrategy}>
            <ul className="type-list">
              {types.map((t) => {
                const category = nameOf(
                  config.categories,
                  t.categoryId || config.settings?.categoryId || '',
                )
                const role = nameOf(
                  config.roles,
                  t.staffRoleId || config.settings?.staffRoleId || '',
                )
                return (
                  <SortableRow key={t.id} id={t.id} name={t.name}>
                    <span className="type-emoji" aria-hidden="true">
                      {t.emoji || '🎫'}
                    </span>
                    <div className="type-main">
                      <strong>{t.name}</strong>
                      {t.description && <span className="type-desc">{t.description}</span>}
                      {(category || role || t.questions.length > 0 || t.aiEnabled) && (
                        <span className="type-chips">
                          {category && <span className="chip">{category}</span>}
                          {role && <span className="chip">@{role}</span>}
                          {t.aiEnabled && <span className="chip">Atiende la IA</span>}
                          {t.questions.length > 0 && (
                            <span className="chip">
                              {t.questions.length}{' '}
                              {t.questions.length === 1 ? 'pregunta' : 'preguntas'}
                            </span>
                          )}
                        </span>
                      )}
                    </div>
                    <div className="type-actions">
                      <Button variant="outline" onClick={() => setEditing(t)}>
                        Editar
                      </Button>
                      <Button
                        variant="danger"
                        onClick={() => setRemoving(t)}
                        disabled={types.length <= 1}
                        title={
                          types.length <= 1 ? 'El servidor necesita al menos un tipo' : undefined
                        }
                      >
                        Eliminar
                      </Button>
                    </div>
                  </SortableRow>
                )
              })}
            </ul>
          </SortableContext>
        </DndContext>
      )}

      <TypeDialog
        key={editing === null ? 'closed' : editing === 'new' ? 'new' : editing.id}
        guildId={guildId}
        editing={editing}
        onClose={() => setEditing(null)}
      />
      <RemoveDialog guildId={guildId} type={removing} onClose={() => setRemoving(null)} />
    </section>
  )
}

// Fila de la lista que se puede arrastrar por su asa (también con teclado: espacio y flechas).
function SortableRow({ id, name, children }: { id: number; name: string; children: ReactNode }) {
  const {
    attributes,
    listeners,
    setNodeRef,
    setActivatorNodeRef,
    transform,
    transition,
    isDragging,
    isSorting,
  } = useSortable({
    id,
    // Sin animar el cambio de sitio al soltar: la fila ya está donde toca.
    animateLayoutChanges: () => false,
  })
  return (
    <li
      ref={setNodeRef}
      className={isDragging ? 'type-row dragging' : 'type-row'}
      style={{
        transform: CSS.Translate.toString(transform),
        // La transición solo mientras se arrastra: al soltar, la fila no debe animarse.
        transition: isSorting ? transition : undefined,
      }}
    >
      <button
        type="button"
        ref={setActivatorNodeRef}
        className="drag-handle"
        aria-label={`Reordenar ${name}`}
        {...attributes}
        {...listeners}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor" aria-hidden="true">
          <circle cx="4" cy="3" r="1.2" />
          <circle cx="10" cy="3" r="1.2" />
          <circle cx="4" cy="7" r="1.2" />
          <circle cx="10" cy="7" r="1.2" />
          <circle cx="4" cy="11" r="1.2" />
          <circle cx="10" cy="11" r="1.2" />
        </svg>
      </button>
      {children}
    </li>
  )
}

type TypeDialogProps = {
  guildId: string
  editing: TicketType | 'new' | null
  onClose: () => void
}

// Alta o edición de un tipo. Se vuelve a montar al cambiar de tipo (ver key), así el
// formulario siempre empieza con los datos de ese tipo.
function TypeDialog({ guildId, editing, onClose }: TypeDialogProps) {
  const queryClient = useQueryClient()
  const { config } = useSetup()
  const current = editing && editing !== 'new' ? editing : null

  const [form, setForm] = useState<TicketTypeInput>({
    name: current?.name ?? '',
    emoji: current?.emoji ?? '',
    description: current?.description ?? '',
    categoryId: current?.categoryId || config.settings?.categoryId || '',
    staffRoleId: current?.staffRoleId || config.settings?.staffRoleId || '',
    questions: current?.questions ?? [],
    aiEnabled: current?.aiEnabled ?? false,
    aiInstructions: current?.aiInstructions ?? '',
    aiEscalation: current?.aiEscalation ?? '',
  })
  const set = <K extends keyof TicketTypeInput>(key: K, value: TicketTypeInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

  const save = useMutation({
    mutationFn: () =>
      current ? updateTicketType(guildId, current.id, form) : createTicketType(guildId, form),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ticketTypesQuery(guildId).queryKey })
      toast(current ? 'Tipo actualizado' : 'Tipo creado')
      onClose()
    },
  })

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    save.mutate()
  }

  return (
    <Dialog
      open={editing !== null}
      onOpenChange={(open) => !open && onClose()}
      title={current ? 'Editar tipo' : 'Nuevo tipo de ticket'}
      size="lg"
    >
      <form className="type-form" onSubmit={onSubmit}>
        <div className="type-form-row">
          <FormField label="Emoji">
            {({ id }) => (
              <Input
                id={id}
                value={form.emoji}
                onChange={(e) => set('emoji', e.target.value)}
                placeholder="💳"
                maxLength={16}
                className="emoji-input"
              />
            )}
          </FormField>
          <FormField label="Nombre">
            {({ id }) => (
              <Input
                id={id}
                value={form.name}
                onChange={(e) => set('name', e.target.value)}
                placeholder="Pagos y facturación"
                maxLength={80}
                required
                autoFocus
              />
            )}
          </FormField>
        </div>

        <FormField label="Descripción" hint="Una frase para que el miembro sepa cuándo elegirlo.">
          {({ id }) => (
            <Input
              id={id}
              value={form.description}
              onChange={(e) => set('description', e.target.value)}
              placeholder="Problemas con un cobro o una factura"
              maxLength={100}
            />
          )}
        </FormField>

        <FormField label="Categoría" hint="Dónde se crean los tickets de este tipo." custom>
          {({ id, labelId }) => (
            <Select
              id={id}
              labelledBy={labelId}
              value={form.categoryId}
              onChange={(v) => set('categoryId', v)}
              options={config.categories.map((c) => ({ value: c.id, label: c.name }))}
            />
          )}
        </FormField>

        <FormField label="Rol de staff" hint="Quién atiende los tickets de este tipo." custom>
          {({ id, labelId }) => (
            <Select
              id={id}
              labelledBy={labelId}
              value={form.staffRoleId}
              onChange={(v) => set('staffRoleId', v)}
              options={config.roles.map((r) => ({ value: r.id, label: r.name }))}
            />
          )}
        </FormField>

        <QuestionsEditor questions={form.questions} onChange={(q) => set('questions', q)} />

        <AiSection guildId={guildId} form={form} set={set} />

        {save.isError && <small className="error">{save.error.message}</small>}

        <div className="tour-actions">
          <Button variant="outline" onClick={onClose} disabled={save.isPending}>
            Cancelar
          </Button>
          <Button
            type="submit"
            loading={save.isPending}
            disabled={
              !form.name.trim() ||
              !form.categoryId ||
              !form.staffRoleId ||
              form.questions.some((q) => !q.label.trim()) ||
              textLength(form.aiInstructions) > MAX_AI_INSTRUCTIONS ||
              textLength(form.aiEscalation) > MAX_AI_ESCALATION
            }
          >
            {save.isPending ? 'Guardando' : current ? 'Guardar' : 'Crear tipo'}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}

function RemoveDialog({
  guildId,
  type,
  onClose,
}: {
  guildId: string
  type: TicketType | null
  onClose: () => void
}) {
  const queryClient = useQueryClient()

  const remove = useMutation({
    mutationFn: (id: number) => deleteTicketType(guildId, id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ticketTypesQuery(guildId).queryKey })
      toast('Tipo eliminado')
      onClose()
    },
  })

  return (
    <Dialog
      open={type !== null}
      onOpenChange={(open) => !open && onClose()}
      title={`Eliminar "${type?.name ?? ''}"`}
    >
      <p>
        Los miembros dejarán de poder abrir este tipo de ticket. Los tickets que ya existen no se
        ven afectados.
      </p>
      {remove.isError && <small className="error">{remove.error.message}</small>}
      <div className="tour-actions">
        <Button variant="outline" onClick={onClose} disabled={remove.isPending}>
          Cancelar
        </Button>
        <Button
          variant="danger"
          loading={remove.isPending}
          onClick={() => type && remove.mutate(type.id)}
        >
          {remove.isPending ? 'Eliminando' : 'Eliminar tipo'}
        </Button>
      </div>
    </Dialog>
  )
}

// Formulario que ve quien abre un ticket de este tipo: hasta cinco preguntas.
function QuestionsEditor({
  questions,
  onChange,
}: {
  questions: Question[]
  onChange: (questions: Question[]) => void
}) {
  const update = (index: number, patch: Partial<Question>) =>
    onChange(questions.map((q, i) => (i === index ? { ...q, ...patch } : q)))

  return (
    <fieldset className="questions">
      <legend>Formulario</legend>
      <p className="questions-hint">
        Preguntas que se hacen al abrir el ticket. Las respuestas aparecen en el canal del ticket y
        en su transcripción. Déjalo vacío para abrir el ticket directamente.
      </p>

      {questions.map((q, i) => (
        <div key={i} className="question">
          <Input
            value={q.label}
            onChange={(e) => update(i, { label: e.target.value })}
            placeholder="Por ejemplo: ¿Cuál es el número de tu pedido?"
            maxLength={MAX_QUESTION_LABEL}
            aria-label={`Pregunta ${i + 1}`}
          />
          <div className="question-options">
            <Checkbox checked={q.required} onChange={(v) => update(i, { required: v })}>
              Obligatoria
            </Checkbox>
            <Checkbox checked={q.long} onChange={(v) => update(i, { long: v })}>
              Respuesta larga
            </Checkbox>
            <button
              type="button"
              className="icon-btn"
              aria-label={`Quitar la pregunta ${i + 1}`}
              onClick={() => onChange(questions.filter((_, j) => j !== i))}
            >
              ×
            </button>
          </div>
        </div>
      ))}

      {questions.length < MAX_QUESTIONS && (
        <Button
          variant="outline"
          onClick={() => onChange([...questions, { label: '', required: true, long: false }])}
        >
          Añadir pregunta
        </Button>
      )}
    </fieldset>
  )
}

// Caracteres de un texto como los cuenta el servidor: sin los espacios de los extremos y
// con un emoji como uno solo.
const textLength = (text: string) => [...text.trim()].length

// Si el asistente de IA atiende este tipo, cómo, y cuándo pasa el ticket a una persona.
function AiSection({
  guildId,
  form,
  set,
}: {
  guildId: string
  form: TicketTypeInput
  set: <K extends keyof TicketTypeInput>(key: K, value: TicketTypeInput[K]) => void
}) {
  const { data: ai } = useQuery(aiQuery(guildId))

  // Por qué, aunque se active aquí, el asistente aún no atenderá estos tickets.
  let notice: ReactNode = null
  if (ai && !ai.available) {
    notice = 'La IA no está disponible en este servidor de Askord.'
  } else if (ai && !ai.hasKey) {
    notice = (
      <>
        Aún no has conectado un proveedor de IA.{' '}
        <Link to="/guilds/$guildId/ai" params={{ guildId }}>
          Conectarlo
        </Link>
      </>
    )
  } else if (ai && !ai.enabled) {
    notice = (
      <>
        El asistente está desactivado en este servidor.{' '}
        <Link to="/guilds/$guildId/ai" params={{ guildId }}>
          Activarlo
        </Link>
      </>
    )
  }

  return (
    <fieldset className="questions ai-section">
      <legend>Asistente de IA</legend>
      <Checkbox checked={form.aiEnabled} onChange={(v) => set('aiEnabled', v)}>
        Atender los tickets de este tipo con el asistente de IA
      </Checkbox>
      {form.aiEnabled && notice && <p className="questions-hint">{notice}</p>}

      {form.aiEnabled && (
        <>
          <FormField
            label="Instrucciones"
            hint="Qué debe hacer con estos tickets: qué preguntar, qué información dar, qué problemas resolver. Se suman a las pautas generales del servidor. Admite markdown."
          >
            {({ id }) => (
              <Textarea
                id={id}
                value={form.aiInstructions}
                onChange={(v) => set('aiInstructions', v)}
                limit={MAX_AI_INSTRUCTIONS}
                rows={5}
                placeholder="Ayuda con los pagos: pide el número de pedido, explica los plazos de facturación…"
                spellCheck
              />
            )}
          </FormField>

          <FormField
            label="Cuándo pasar a una persona"
            hint="En qué casos el asistente debe dejar de responder y avisar al staff."
          >
            {({ id }) => (
              <Textarea
                id={id}
                value={form.aiEscalation}
                onChange={(v) => set('aiEscalation', v)}
                limit={MAX_AI_ESCALATION}
                rows={3}
                placeholder="Si piden un reembolso, si el usuario está muy molesto o si no sabes resolverlo…"
                spellCheck
              />
            )}
          </FormField>
        </>
      )}
    </fieldset>
  )
}
