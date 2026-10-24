import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import '../components/transcripts.css'
import { DiscordFrame, DiscordMessage } from '../components/discord/Discord'
import { TranscriptDetailSkeleton } from '../components/skeletons'
import { buttonClassName } from '../components/ui/button-styles'
import { discordAvatarUrl, formatDateTime, ticketNumber } from '../lib/format'
import { ticketExportUrl, transcriptQuery, type ExportFormat } from '../lib/transcripts'

export const Route = createFileRoute('/guilds/$guildId/transcripts/$ticketId')({
  // Precarga sin esperar: se entra al instante y la página enseña un esqueleto.
  loader: ({ context, params }) => {
    void context.queryClient.prefetchQuery(transcriptQuery(params.guildId, params.ticketId))
  },
  component: TranscriptDetail,
})

const FORMATS: { format: ExportFormat; label: string }[] = [
  { format: 'txt', label: 'TXT' },
  { format: 'csv', label: 'CSV' },
  { format: 'json', label: 'JSON' },
]

function TranscriptDetail() {
  const { guildId, ticketId } = Route.useParams()
  const { data: t, isPending, isError } = useQuery(transcriptQuery(guildId, ticketId))

  if (isPending) return <TranscriptDetailSkeleton />
  if (isError) {
    return (
      <div>
        <Link to="/guilds/$guildId/transcripts" params={{ guildId }}>
          ← Transcripciones
        </Link>
        <p className="error">No se pudo cargar esta transcripción.</p>
      </div>
    )
  }
  const number = ticketNumber(t.number)

  return (
    <div>
      <Link to="/guilds/$guildId/transcripts" params={{ guildId }}>
        ← Transcripciones
      </Link>

      <div className="section-head">
        <h1>Ticket #{number}</h1>
        <div className="export">
          <span>Exportar</span>
          {FORMATS.map(({ format, label }) => (
            <a
              key={format}
              className={buttonClassName('outline')}
              href={ticketExportUrl(guildId, t.id, format)}
              download
            >
              {label}
            </a>
          ))}
        </div>
      </div>
      <p>
        {t.typeName && <span className="chip">{`${t.typeEmoji} ${t.typeName}`.trim()}</span>}{' '}
        Abierto por <strong>{t.openerName}</strong> el {formatDateTime(t.createdAt)}
        {t.closedAt && ` · cerrado el ${formatDateTime(t.closedAt)}`}
      </p>

      <DiscordFrame channel={{ name: `ticket-${number}`, locked: true }}>
        <div className="conversation">
          {t.conversation.map((m) => (
            <DiscordMessage
              key={m.id}
              author={m.authorName}
              avatarUrl={discordAvatarUrl(m.authorId, m.authorAvatar)}
              time={formatDateTime(m.createdAt)}
            >
              {m.content && <div className="dc-text">{m.content}</div>}
              {m.attachments.map((url) => (
                <a key={url} className="dc-attachment" href={url} target="_blank" rel="noreferrer">
                  {url.split('/').pop()?.split('?')[0] || 'Adjunto'}
                </a>
              ))}
            </DiscordMessage>
          ))}
        </div>
      </DiscordFrame>
    </div>
  )
}
