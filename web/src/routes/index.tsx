import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { avatarUrl, logout, meQuery } from '../lib/auth'

export const Route = createFileRoute('/')({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meQuery)
    if (!user) throw redirect({ to: '/login' })
  },
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
      <p>Sesión iniciada. Aquí irá la lista de tus servidores.</p>
    </main>
  )
}
