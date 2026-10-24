import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, redirect } from '@tanstack/react-router'
import { AppShell } from '../components/AppShell'
import { Button } from '../components/ui/Button'
import { Spinner } from '../components/ui/Spinner'
import { buttonClassName } from '../components/ui/button-styles'
import { requireUser } from '../lib/auth'
import { guildIconUrl, guildsQuery, type Guild } from '../lib/guilds'
import { setPendingInvite } from '../lib/pendingInvite'

export const Route = createFileRoute('/')({
  beforeLoad: async ({ context }) => {
    await requireUser(context.queryClient)

    // Con algún servidor configurado, el inicio es el panel de ese servidor.
    // Esta lista solo se ve mientras no hay ninguno.
    const guilds = await context.queryClient.fetchQuery(guildsQuery).catch(() => [])
    const first = guilds.find((g) => g.botInstalled)
    if (first) throw redirect({ to: '/guilds/$guildId', params: { guildId: first.id } })
  },
  component: Dashboard,
})

function Dashboard() {
  return (
    <AppShell>
      <Guilds />
    </AppShell>
  )
}

function Guilds() {
  const { data: guilds, isPending, isError, isFetching, refetch } = useQuery(guildsQuery)

  if (isPending) {
    return (
      <p className="loading">
        <Spinner /> Cargando servidores…
      </p>
    )
  }
  if (isError) return <p className="error">No se pudieron cargar los servidores.</p>

  return (
    <section>
      <div className="section-head">
        <h2>Tus servidores</h2>
        <Button variant="outline" onClick={() => refetch()} loading={isFetching}>
          Actualizar
        </Button>
      </div>
      {guilds.length === 0 ? (
        <p>
          No administras ningún servidor. Si acabas de crear uno, cierra sesión y vuelve a entrar
          para actualizar la lista.
        </p>
      ) : (
        <ul className="guilds">
          {guilds.map((g) => (
            <GuildCard key={g.id} guild={g} />
          ))}
        </ul>
      )}
    </section>
  )
}

function GuildCard({ guild }: { guild: Guild }) {
  const icon = guildIconUrl(guild)
  return (
    <li className="guild">
      {icon ? (
        <img src={icon} alt="" width={40} height={40} />
      ) : (
        <span className="guild-fallback">{guild.name.charAt(0).toUpperCase()}</span>
      )}
      <span className="guild-name">{guild.name}</span>
      {guild.botInstalled ? (
        <>
          <span className="badge">Bot activo</span>
          <Link className={buttonClassName()} to="/guilds/$guildId" params={{ guildId: guild.id }}>
            Configurar
          </Link>
        </>
      ) : (
        <a
          className={buttonClassName()}
          href={guild.inviteUrl}
          onClick={() => setPendingInvite(guild.id)}
          target="_blank"
          rel="noreferrer"
        >
          Añadir bot
        </a>
      )}
    </li>
  )
}
