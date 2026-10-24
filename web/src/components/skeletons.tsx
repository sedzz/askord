import type { ReactNode } from 'react'
import { Skeleton } from './ui/Skeleton'

// Esqueletos de cada pantalla: tienen la forma de lo que va a aparecer, para que el
// contenido no salte al llegar. Un solo "Cargando" oculto avisa a los lectores de pantalla.

function Loading({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={className} role="status" aria-busy="true">
      <span className="visually-hidden">Cargando…</span>
      {children}
    </div>
  )
}

// Título y unas líneas: para cualquier página sin forma propia.
export function ContentSkeleton() {
  return (
    <Loading className="skeleton-stack">
      <Skeleton width={220} height={32} />
      <Skeleton width="60%" height={16} />
      <Skeleton width="45%" height={16} />
    </Loading>
  )
}

// Marco de un servidor: sub-barra lateral y contenido.
export function GuildFrameSkeleton() {
  return (
    <Loading className="guild-layout">
      <div className="subnav">
        <div className="subnav-title">
          <Skeleton width={24} height={24} radius="50%" />
          <Skeleton width={110} height={16} />
        </div>
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} height={36} />
        ))}
      </div>
      <div className="guild-page skeleton-stack">
        <Skeleton width={220} height={32} />
        <Skeleton height={120} radius={12} />
        <Skeleton height={120} radius={12} />
      </div>
    </Loading>
  )
}

export function DashboardSkeleton() {
  return (
    <Loading className="dashboard">
      <Skeleton width={180} height={32} />
      <div className="card preview-card">
        <div className="preview-head">
          <Skeleton width={64} height={64} radius={16} />
          <div className="skeleton-stack">
            <Skeleton width={180} height={20} />
            <Skeleton width={120} height={14} />
          </div>
        </div>
        <div className="preview-stats">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="skeleton-stack">
              <Skeleton width={70} height={12} />
              <Skeleton width={90} height={22} />
            </div>
          ))}
        </div>
      </div>
      <div className="stats">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="card stat">
            <Skeleton width={120} height={12} />
            <Skeleton width={70} height={30} />
          </div>
        ))}
      </div>
      <div className="card">
        <Skeleton width={200} height={20} />
        <Skeleton height={200} className="skeleton-gap" />
      </div>
    </Loading>
  )
}

// Filas de la lista de transcripciones.
export function TranscriptRowsSkeleton() {
  return (
    <Loading className="skeleton-stack">
      {[0, 1, 2, 3, 4].map((i) => (
        <Skeleton key={i} height={48} />
      ))}
    </Loading>
  )
}

export function TranscriptDetailSkeleton() {
  return (
    <Loading className="skeleton-stack">
      <Skeleton width={140} height={16} />
      <Skeleton width={200} height={32} />
      <Skeleton width="50%" height={16} />
      <div className="skeleton-stack skeleton-gap">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="skeleton-message">
            <Skeleton width={40} height={40} radius="50%" />
            <div className="skeleton-stack">
              <Skeleton width={140} height={14} />
              <Skeleton width={`${70 - i * 8}%`} height={14} />
            </div>
          </div>
        ))}
      </div>
    </Loading>
  )
}

// Filas de servidores (lista inicial y diálogo de añadir servidor).
export function GuildRowsSkeleton() {
  return (
    <Loading className="guilds">
      {[0, 1, 2].map((i) => (
        <div key={i} className="guild">
          <Skeleton width={40} height={40} radius="50%" />
          <Skeleton height={16} width="45%" className="skeleton-grow" />
          <Skeleton width={96} height={36} />
        </div>
      ))}
    </Loading>
  )
}
