import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import '../components/transcripts.css'
import { Button } from '../components/ui/Button'
import { buttonClassName } from '../components/ui/button-styles'
import { Input } from '../components/ui/Input'
import { Spinner } from '../components/ui/Spinner'
import { formatDate, ticketNumber } from '../lib/format'
import {
  exportUrl,
  transcriptsQuery,
  type ExportFormat,
  type TranscriptFilters,
} from '../lib/transcripts'

const DATE = /^\d{4}-\d{2}-\d{2}$/

export const Route = createFileRoute('/guilds/$guildId/transcripts/')({
  // Los filtros viven en la dirección: se pueden compartir y sobreviven a recargar.
  validateSearch: (search: Record<string, unknown>): TranscriptFilters => {
    const text = (v: unknown) => (typeof v === 'string' && v.trim() ? v.trim() : undefined)
    const date = (v: unknown) => (typeof v === 'string' && DATE.test(v) ? v : undefined)
    return { from: date(search.from), to: date(search.to), user: text(search.user) }
  },
  component: Transcripts,
})

const FORMATS: { format: ExportFormat; label: string }[] = [
  { format: 'csv', label: 'CSV' },
  { format: 'json', label: 'JSON' },
  { format: 'txt', label: 'TXT' },
]

function Transcripts() {
  const { guildId } = Route.useParams()
  const filters = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data, isPending, isError, isFetchingNextPage, hasNextPage, fetchNextPage } =
    useInfiniteQuery({
      ...transcriptsQuery(guildId, filters),
      // Al cambiar un filtro se mantiene la lista anterior hasta que llega la nueva.
      placeholderData: keepPreviousData,
    })

  const setFilter = (key: keyof TranscriptFilters, value: string) =>
    navigate({ search: (prev) => ({ ...prev, [key]: value || undefined }), replace: true })

  // El usuario se escribe a mano: se espera un momento antes de buscar.
  const [userInput, setUserInput] = useState(filters.user ?? '')
  useEffect(() => {
    const id = setTimeout(() => {
      const next = userInput.trim() || undefined
      if (next !== filters.user) {
        void navigate({ search: (prev) => ({ ...prev, user: next }), replace: true })
      }
    }, 300)
    return () => clearTimeout(id)
  }, [userInput, filters.user, navigate])

  const hasFilters = Boolean(filters.from || filters.to || filters.user)
  const items = data?.pages.flatMap((p) => p.items) ?? []

  function clearFilters() {
    setUserInput('')
    void navigate({ search: {}, replace: true })
  }

  return (
    <div>
      <div className="section-head">
        <h1>Transcripciones</h1>
        {items.length > 0 && (
          <div className="export">
            <span>Exportar</span>
            {FORMATS.map(({ format, label }) => (
              <a
                key={format}
                className={buttonClassName('outline')}
                href={exportUrl(guildId, filters, format)}
                download
              >
                {label}
              </a>
            ))}
          </div>
        )}
      </div>
      <p>Conversaciones guardadas de los tickets de este servidor.</p>

      <div className="filters">
        <label className="filter">
          <span>Desde</span>
          <Input
            type="date"
            value={filters.from ?? ''}
            max={filters.to}
            onChange={(e) => setFilter('from', e.target.value)}
          />
        </label>
        <label className="filter">
          <span>Hasta</span>
          <Input
            type="date"
            value={filters.to ?? ''}
            min={filters.from}
            onChange={(e) => setFilter('to', e.target.value)}
          />
        </label>
        <label className="filter filter-grow">
          <span>Usuario</span>
          <Input
            type="search"
            placeholder="Nombre o ID de quien participó"
            value={userInput}
            onChange={(e) => setUserInput(e.target.value)}
          />
        </label>
        {hasFilters && (
          <Button variant="outline" onClick={clearFilters}>
            Limpiar
          </Button>
        )}
      </div>

      {isPending ? (
        <p className="loading">
          <Spinner /> Cargando transcripciones…
        </p>
      ) : isError ? (
        <p className="error">No se pudieron cargar las transcripciones.</p>
      ) : items.length === 0 ? (
        <p className="hint">
          {hasFilters
            ? 'Ninguna transcripción coincide con estos filtros.'
            : 'Todavía no hay transcripciones. Se guardan al pulsar Transcribir en un ticket cerrado.'}
        </p>
      ) : (
        <>
          <ul className="transcripts">
            {items.map((t) => (
              <li key={t.id}>
                <Link
                  to="/guilds/$guildId/transcripts/$ticketId"
                  params={{ guildId, ticketId: String(t.id) }}
                  className="transcript-row"
                >
                  <span className="transcript-number">#{ticketNumber(t.number)}</span>
                  <span className="transcript-user">{t.openerName}</span>
                  <span className="transcript-preview">{t.preview}</span>
                  <span className="transcript-meta">{formatDate(t.createdAt)}</span>
                  <span className="transcript-meta">
                    {t.messages} {t.messages === 1 ? 'mensaje' : 'mensajes'}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
          {hasNextPage && (
            <Button variant="outline" onClick={() => fetchNextPage()} loading={isFetchingNextPage}>
              Cargar más
            </Button>
          )}
        </>
      )}
    </div>
  )
}
