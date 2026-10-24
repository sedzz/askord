import { useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { requireUser } from '../lib/auth'
import {
  createCategory,
  createRole,
  MISSING_PERMISSIONS,
  saveTicketSettings,
  ticketConfigQuery,
  type Option,
  type TicketConfig,
} from '../lib/tickets'

export const Route = createFileRoute('/guilds/$guildId')({
  beforeLoad: ({ context }) => requireUser(context.queryClient),
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(ticketConfigQuery(params.guildId)),
  errorComponent: () => (
    <main className="page">
      <Link to="/">← Volver</Link>
      <p className="error">No se pudo cargar este servidor.</p>
    </main>
  ),
  component: GuildSettings,
})

function GuildSettings() {
  const { guildId } = Route.useParams()
  const { data } = useSuspenseQuery(ticketConfigQuery(guildId))
  const queryClient = useQueryClient()
  const queryKey = ticketConfigQuery(guildId).queryKey

  const [categoryId, setCategoryId] = useState(data.settings?.categoryId ?? '')
  const [staffRoleId, setStaffRoleId] = useState(data.settings?.staffRoleId ?? '')

  const save = useMutation({
    mutationFn: () => saveTicketSettings(guildId, { categoryId, staffRoleId }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey }),
  })

  // Añade lo recién creado a la lista y lo deja seleccionado.
  function addOption(field: 'categories' | 'roles', option: Option) {
    queryClient.setQueryData<TicketConfig>(
      queryKey,
      (old) => old && { ...old, [field]: [...old[field], option] },
    )
  }

  const makeCategory = useMutation({
    mutationFn: () => createCategory(guildId),
    onSuccess: (option) => {
      addOption('categories', option)
      setCategoryId(option.id)
    },
  })

  const makeRole = useMutation({
    mutationFn: () => createRole(guildId),
    onSuccess: (option) => {
      addOption('roles', option)
      setStaffRoleId(option.id)
    },
  })

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    save.mutate()
  }

  return (
    <main className="page">
      <Link to="/">← Volver</Link>
      <h1>Tickets</h1>
      <p>Elige dónde se crearán los tickets y quién podrá verlos.</p>

      <form className="form" onSubmit={onSubmit}>
        <Select
          label="Categoría de tickets"
          value={categoryId}
          options={data.categories}
          onChange={setCategoryId}
          createLabel="Crear categoría"
          onCreate={() => makeCategory.mutate()}
          creating={makeCategory.isPending}
          createError={makeCategory.error}
        />
        <Select
          label="Rol de staff"
          value={staffRoleId}
          options={data.roles}
          onChange={setStaffRoleId}
          createLabel="Crear rol"
          onCreate={() => makeRole.mutate()}
          creating={makeRole.isPending}
          createError={makeRole.error}
        />

        <div className="form-actions">
          <button type="submit" disabled={!categoryId || !staffRoleId || save.isPending}>
            Guardar
          </button>
          {save.isSuccess && <span className="saved">Guardado</span>}
          {save.isError && <span className="error">No se pudo guardar</span>}
        </div>
      </form>
    </main>
  )
}

type SelectProps = {
  label: string
  value: string
  options: Option[]
  onChange: (value: string) => void
  createLabel: string
  onCreate: () => void
  creating: boolean
  createError: Error | null
}

function Select({
  label,
  value,
  options,
  onChange,
  createLabel,
  onCreate,
  creating,
  createError,
}: SelectProps) {
  return (
    <div className="field">
      <label htmlFor={label}>{label}</label>
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
              {o.name}
            </option>
          ))}
        </select>
        <button type="button" onClick={onCreate} disabled={creating}>
          {createLabel}
        </button>
      </div>
      {createError && (
        <small className="error">
          {createError.message === MISSING_PERMISSIONS
            ? 'El bot no tiene permisos. Dale "Gestionar canales" y "Gestionar roles" en Discord, o vuelve a invitarlo desde el panel.'
            : 'No se pudo crear. Inténtalo de nuevo.'}
        </small>
      )}
    </div>
  )
}
