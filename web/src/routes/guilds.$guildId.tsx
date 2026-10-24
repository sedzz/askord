import { useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { PanelSection } from '../components/PanelSection'
import { Select } from '../components/Select'
import { AppShell } from '../components/AppShell'
import { Button } from '../components/ui/Button'
import { requireUser } from '../lib/auth'
import { guildsQuery } from '../lib/guilds'
import {
  createCategory,
  createRole,
  saveTicketSettings,
  ticketConfigQuery,
  type Option,
  type TicketConfig,
} from '../lib/tickets'

export const Route = createFileRoute('/guilds/$guildId')({
  beforeLoad: ({ context }) => requireUser(context.queryClient),
  loader: ({ context, params }) =>
    Promise.all([
      context.queryClient.ensureQueryData(ticketConfigQuery(params.guildId)),
      context.queryClient.ensureQueryData(guildsQuery),
    ]),
  errorComponent: () => (
    <main className="page">
      <Link to="/">← Volver</Link>
      <p className="error">No se pudo cargar este servidor.</p>
    </main>
  ),
  component: GuildPage,
})

function GuildPage() {
  const { guildId } = Route.useParams()
  return (
    <AppShell activeGuildId={guildId}>
      {/* key: al cambiar de servidor se reinicia el formulario con sus datos. */}
      <GuildSettings key={guildId} guildId={guildId} />
    </AppShell>
  )
}

function GuildSettings({ guildId }: { guildId: string }) {
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
    <div>
      <h1>Tickets</h1>
      <p>Sigue los tres pasos para que tus miembros puedan abrir tickets desde Discord.</p>

      <form className="form" onSubmit={onSubmit}>
        <Select
          label="1. Rol de staff"
          help="Las personas con este rol verán y atenderán todos los tickets."
          value={staffRoleId}
          options={data.roles}
          onChange={setStaffRoleId}
          createLabel="Crear rol"
          onCreate={() => makeRole.mutate()}
          creating={makeRole.isPending}
          createError={makeRole.error}
          missingPermissionsHint='El bot no tiene permisos. Dale "Gestionar roles" en Discord, o vuelve a invitarlo desde el panel.'
        />
        <Select
          label="2. Categoría de tickets"
          help="Aquí se crearán los canales privados de cada ticket (ticket-0001, ticket-0002…)."
          value={categoryId}
          options={data.categories}
          onChange={setCategoryId}
          createLabel="Crear categoría"
          onCreate={() => makeCategory.mutate()}
          creating={makeCategory.isPending}
          createError={makeCategory.error}
          missingPermissionsHint='El bot no tiene permisos. Dale "Gestionar canales" en Discord, o vuelve a invitarlo desde el panel.'
        />

        <div className="form-actions">
          <Button type="submit" disabled={!categoryId || !staffRoleId || save.isPending}>
            Guardar
          </Button>
          {save.isSuccess && <span className="saved">Guardado</span>}
          {save.isError && <span className="error">No se pudo guardar</span>}
        </div>
      </form>

      {data.settings ? (
        <PanelSection
          guildId={guildId}
          channels={data.textChannels}
          currentChannelId={data.settings.panelChannelId}
        />
      ) : (
        <p className="hint">Guarda los pasos 1 y 2 para poder publicar el panel (paso 3).</p>
      )}
    </div>
  )
}
