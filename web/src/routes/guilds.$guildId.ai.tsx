import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { AiConnection, AiStatus } from '../components/AiConnection'
import { Skeleton } from '../components/ui/Skeleton'
import { aiQuery } from '../lib/ai'

export const Route = createFileRoute('/guilds/$guildId/ai')({
  component: AiSettings,
})

function AiSettings() {
  const { guildId } = Route.useParams()
  const { data, isPending, isError } = useQuery(aiQuery(guildId))
  const [editOpen, setEditOpen] = useState(false)

  return (
    <div className="settings-page ai-page">
      <div className="section-head">
        <h1>Asistente de IA</h1>
        {data?.available && data.hasKey && (
          <AiStatus connection={data} onEdit={() => setEditOpen(true)} />
        )}
      </div>
      <p>
        Conecta tu propio proveedor de IA. Tus claves se guardan cifradas y el coste de las
        peticiones corre por tu cuenta.
      </p>

      {isPending && <Skeleton width={560} height={260} radius={12} className="ai-skeleton" />}
      {isError && <p className="error">No se pudo cargar la configuración.</p>}
      {data && !data.available && (
        <p className="hint">La IA no está disponible en este servidor de Askord.</p>
      )}
      {data?.available && (
        <AiConnection
          guildId={guildId}
          connection={data}
          editOpen={editOpen}
          onEditOpenChange={setEditOpen}
        />
      )}
    </div>
  )
}
