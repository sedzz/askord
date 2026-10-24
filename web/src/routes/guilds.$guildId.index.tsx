import { createFileRoute, Link } from '@tanstack/react-router'
import { buttonClassName } from '../components/ui/button-styles'
import { useSetup } from '../lib/ticketSetupContext'

export const Route = createFileRoute('/guilds/$guildId/')({
  component: Dashboard,
})

function Dashboard() {
  const { guildId } = Route.useParams()
  const { config } = useSetup()
  const settings = config.settings

  const name = (list: { id: string; name: string }[], id?: string) =>
    list.find((o) => o.id === id)?.name

  const steps = [
    { label: 'Rol de staff', value: name(config.roles, settings?.staffRoleId) },
    { label: 'Categoría de tickets', value: name(config.categories, settings?.categoryId) },
    {
      label: 'Panel de tickets',
      value: settings?.panelChannelId
        ? `# ${name(config.textChannels, settings.panelChannelId) ?? ''}`
        : undefined,
    },
  ]
  const complete = steps.every((s) => s.value)

  return (
    <div>
      <h1>Dashboard</h1>
      <p>
        {complete
          ? 'Tu sistema de tickets está listo y funcionando.'
          : 'Falta configurar algo para que tus miembros puedan abrir tickets.'}
      </p>

      <ul className="checklist">
        {steps.map((s) => (
          <li key={s.label} className={s.value ? 'check done' : 'check'}>
            <span className="check-mark" aria-hidden="true">
              {s.value && (
                <svg viewBox="0 0 24 24" fill="none">
                  <path
                    d="M20 6 9 17l-5-5"
                    stroke="currentColor"
                    strokeWidth="3.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              )}
            </span>
            <span className="check-label">{s.label}</span>
            <span className="check-value">{s.value ?? 'Pendiente'}</span>
          </li>
        ))}
      </ul>

      {!complete && (
        <Link to="/guilds/$guildId/settings" params={{ guildId }} className={buttonClassName()}>
          Ir a Configuración
        </Link>
      )}
    </div>
  )
}
