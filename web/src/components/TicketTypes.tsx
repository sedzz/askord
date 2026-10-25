import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState, type FormEvent, type ReactNode } from 'react'
import { aiQuery, MAX_AI_ESCALATION, MAX_AI_INSTRUCTIONS } from '../lib/ai'
import { useSetup } from '../lib/ticketSetupContext'
import { toast } from '../lib/toast'
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
} from '../lib/ticketTypes'
import './ticket-types.css'
import { Button } from './ui/Button'
import { Checkbox } from './ui/Checkbox'
import { Dialog } from './ui/Dialog'
import { FormField } from './ui/FormField'
import { Input } from './ui/Input'
import { Select } from './ui/Select'
import { Skeleton } from './ui/Skeleton'
import { Textarea } from './ui/Textarea'

// Tipos de ticket del servidor: lista, orden, alta, edición y borrado.
export function TicketTypes({ guildId }: { guildId: string }) {
  const queryClient = useQueryClient()
  const queryKey = ticketTypesQuery(guildId).queryKey
  const { data: types, isPending, isError } = useQuery(ticketTypesQuery(guildId))
  const { config } = useSetup()

  // null: ningún diálogo abierto; 'new': crear; un tipo: editarlo o borrarlo.
  const [editing, setEditing] = useState<TicketType | 'new' | null>(null)
  const [removing, setRemoving] = useState<TicketType | null>(null)

  const reorder = useMutation({
    mutationFn: (ids: number[]) => reorderTicketTypes(guildId, ids),
    // Se reordena al instante en pantalla; si falla, se vuelve a pedir el orden real.
    onMutate: (ids) => {
      queryClient.setQueryData<TicketType[]>(queryKey, (old) =>
        old ? ids.map((id) => old.find((t) => t.id === id)).filter((t) => t !== undefined) : old,
      )
    },
    onError: () => {
      toast('No se pudo cambiar el orden', 'error')
      void queryClient.invalidateQueries({ queryKey })
    },
  })

  function move(index: number, by: -1 | 1) {
    if (!types) return
    const ids = types.map((t) => t.id)
    const target = index + by
    if (target < 0 || target >= ids.length) return
    ;[ids[index], ids[target]] = [ids[target], ids[index]]
    reorder.mutate(ids)
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

      {isPending ? (
        <div className="skeleton-stack">
          <Skeleton height={56} />
          <Skeleton height={56} />
        </div>
      ) : isError ? (
        <p className="error">No se pudieron cargar los tipos.</p>
      ) : (
        <ul className="type-list">
          {types.map((t, i) => {
            const category = nameOf(
              config.categories,
              t.categoryId || config.settings?.categoryId || '',
            )
            const role = nameOf(config.roles, t.staffRoleId || config.settings?.staffRoleId || '')
            return (
              <li key={t.id} className="type-row">
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
                          {t.questions.length} {t.questions.length === 1 ? 'pregunta' : 'preguntas'}
                        </span>
                      )}
                    </span>
                  )}
                </div>
                <div className="type-actions">
                  <button
                    type="button"
                    className="icon-btn"
                    aria-label={`Subir ${t.name}`}
                    onClick={() => move(i, -1)}
                    disabled={i === 0}
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    className="icon-btn"
                    aria-label={`Bajar ${t.name}`}
                    onClick={() => move(i, 1)}
                    disabled={i === types.length - 1}
                  >
                    ↓
                  </button>
                  <Button variant="outline" onClick={() => setEditing(t)}>
                    Editar
                  </Button>
                  <Button
                    variant="outline"
                    onClick={() => setRemoving(t)}
                    disabled={types.length <= 1}
                    title={types.length <= 1 ? 'El servidor necesita al menos un tipo' : undefined}
                  >
                    Eliminar
                  </Button>
                </div>
              </li>
            )
          })}
        </ul>
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
        <Link to="/guilds/$guildId/settings/ai" params={{ guildId }}>
          Conectarlo
        </Link>
      </>
    )
  } else if (ai && !ai.enabled) {
    notice = (
      <>
        El asistente está desactivado en este servidor.{' '}
        <Link to="/guilds/$guildId/settings/ai" params={{ guildId }}>
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
