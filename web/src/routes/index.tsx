import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { avatarUrl, logout, meQuery, requireUser } from '../lib/auth'
import { guildIconUrl, guildsQuery, type Guild } from '../lib/guilds'

export const Route = createFileRoute('/')({
  beforeLoad: ({ context }) => requireUser(context.queryClient),
  component: Dashboard,
})

function Dashboard() {
  const { data: user } = useQuery(meQuery)
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: async () => {
      queryClient.setQueryData(meQuery.queryKey, null)
      await navigate({ to: '/login' })
    },
  })

  if (!user) return null
  const avatar = avatarUrl(user)

  return (
    <main className="page">
      <header className="topbar">
        <h1>Askord</h1>
        <div className="user">
          {avatar && <img src={avatar} alt="" width={32} height={32} />}
          <span>{user.username}</span>
          <button onClick={() => logoutMutation.mutate()} disabled={logoutMutation.isPending}>
            Salir
          </button>
        </div>
      </header>
      <Guilds />
    </main>
  )
}

function Guilds() {
  const { data: guilds, isPending, isError, isFetching, refetch } = useQuery(guildsQuery)

  if (isPending) return <p>Cargando servidores…</p>
  if (isError) return <p className="error">No se pudieron cargar los servidores.</p>

  return (
    <section>
      <div className="section-head">
        <h2>Tus servidores</h2>
        <button onClick={() => refetch()} disabled={isFetching}>
          Actualizar
        </button>
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
          <Link className="button" to="/guilds/$guildId" params={{ guildId: guild.id }}>
            Configurar
          </Link>
        </>
      ) : (
        <a className="button" href={guild.inviteUrl} target="_blank" rel="noreferrer">
          Añadir bot
        </a>
      )}
    </li>
  )
}
