import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { AiConnection } from '../components/AiConnection'
import { Skeleton } from '../components/ui/Skeleton'
import { aiQuery } from '../lib/ai'

export const Route = createFileRoute('/guilds/$guildId/settings/ai')({
  component: AiSettings,
})

function AiSettings() {
  const { guildId } = Route.useParams()
  const { data, isPending, isError } = useQuery(aiQuery(guildId))

  return (
    <div className="settings-page">
      <h1>Asistente de IA</h1>
      <p>
        Conecta tu propio proveedor de IA. Tus claves se guardan cifradas y el coste de las
        peticiones corre por tu cuenta.
      </p>

      {isPending && <Skeleton width={560} height={260} radius={12} className="ai-skeleton" />}
      {isError && <p className="error">No se pudo cargar la configuración.</p>}
      {data && !data.available && (
        <p className="hint">La IA no está disponible en este servidor de Askord.</p>
      )}
      {data?.available && <AiConnection guildId={guildId} connection={data} />}
    </div>
  )
}
