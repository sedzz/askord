import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import '../components/dashboard.css'
import { BarChart } from '../components/ui/BarChart'
import { SegmentedControl } from '../components/ui/SegmentedControl'
import { DashboardSkeleton } from '../components/skeletons'
import { formatDate, formatDay, formatDuration, formatNumber } from '../lib/format'
import { overviewQuery, type GuildOverview, type Range } from '../lib/overview'

export const Route = createFileRoute('/guilds/$guildId/')({
  component: Dashboard,
})

const RANGES: { value: Range; label: string }[] = [
  { value: 7, label: '7 días' },
  { value: 30, label: '30 días' },
  { value: 90, label: '90 días' },
]

function Dashboard() {
  const { guildId } = Route.useParams()
  const [days, setDays] = useState<Range>(30)
  const { data, isPending, isError } = useQuery({
    ...overviewQuery(guildId, days),
    // Al cambiar de rango se mantiene la gráfica anterior hasta que llega la nueva.
    placeholderData: keepPreviousData,
  })

  if (isPending) return <DashboardSkeleton />
  if (isError) return <p className="error">No se pudo cargar el dashboard.</p>

  const { guild, tickets, series } = data
  const openedInRange = series.reduce((sum, p) => sum + p.opened, 0)

  return (
    <div className="dashboard">
      <h1>Dashboard</h1>

      <GuildPreview guildId={guildId} guild={guild} />

      <section aria-label="Tickets" className="stats">
        <Stat label="Tickets abiertos en total" value={formatNumber(tickets.total)} />
        <Stat label="Sin cerrar ahora" value={formatNumber(tickets.open)} />
        <Stat label="Cerrados" value={formatNumber(tickets.closed)} />
        <Stat
          label="Tiempo medio de cierre"
          value={tickets.closed > 0 ? formatDuration(tickets.avgResolutionSeconds) : '—'}
        />
      </section>

      <section className="card">
        <div className="section-head">
          <div>
            <h2>Tickets abiertos por día</h2>
            <p className="card-sub">
              {formatNumber(openedInRange)} {openedInRange === 1 ? 'ticket' : 'tickets'} en los
              últimos {days} días
            </p>
          </div>
          <SegmentedControl
            label="Rango de la gráfica"
            value={days}
            onChange={setDays}
            options={RANGES}
          />
        </div>
        <BarChart
          data={series.map((p) => ({ label: formatDay(p.date), value: p.opened }))}
          valueLabel={(n) => `${n} ${n === 1 ? 'ticket' : 'tickets'}`}
          ariaLabel={`Tickets abiertos por día durante los últimos ${days} días: ${openedInRange} en total`}
        />
      </section>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="card stat">
      <span className="stat-label">{label}</span>
      <strong className="stat-value">{value}</strong>
    </div>
  )
}

// Vista previa del servidor: icono, nombre y cifras de un vistazo.
function GuildPreview({ guildId, guild }: { guildId: string; guild: GuildOverview | null }) {
  if (!guild) {
    return (
      <section className="card">
        <p className="hint">No se pudo consultar Discord para mostrar los datos del servidor.</p>
      </section>
    )
  }

  const icon = guild.icon
    ? `https://cdn.discordapp.com/icons/${guildId}/${guild.icon}.png?size=128`
    : null

  return (
    <section className="card preview-card" aria-label="Servidor">
      <div className="preview-head">
        {icon ? (
          <img src={icon} alt="" width={64} height={64} />
        ) : (
          <span className="preview-fallback">{guild.name.charAt(0).toUpperCase()}</span>
        )}
        <div>
          <h2>{guild.name}</h2>
          <p className="card-sub">Creado el {formatDate(guild.createdAt)}</p>
        </div>
      </div>

      <dl className="preview-stats">
        <div>
          <dt>Miembros</dt>
          <dd>{formatNumber(guild.memberCount)}</dd>
        </div>
        <div>
          <dt>En línea</dt>
          <dd>
            <span className="online-dot" aria-hidden="true" />
            {formatNumber(guild.onlineCount)}
          </dd>
        </div>
        <div>
          <dt>Canales</dt>
          <dd>
            {formatNumber(guild.textChannels)} texto · {formatNumber(guild.voiceChannels)} voz
          </dd>
        </div>
        <div>
          <dt>Roles</dt>
          <dd>{formatNumber(guild.roles)}</dd>
        </div>
      </dl>
    </section>
  )
}
