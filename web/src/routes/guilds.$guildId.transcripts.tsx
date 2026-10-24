import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/guilds/$guildId/transcripts')({
  component: Transcripts,
})

function Transcripts() {
  return (
    <div>
      <h1>Transcripciones</h1>
      <p>Aquí verás las conversaciones guardadas de los tickets de este servidor.</p>
      <p className="hint">Todavía no hay nada que mostrar.</p>
    </div>
  )
}
