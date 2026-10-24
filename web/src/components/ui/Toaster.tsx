import { dismissToast, useToasts } from '../../lib/toast'
import './toaster.css'

// Pinta las notificaciones. Se monta una sola vez, en la raíz de la app.
export function Toaster() {
  const toasts = useToasts()

  return (
    <div className="ui-toaster" role="status" aria-live="polite">
      {toasts.map((t) => (
        <div key={t.id} className={`ui-toast ui-toast--${t.tone}`}>
          <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
            {t.tone === 'success' ? (
              <path
                d="M20 6 9 17l-5-5"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            ) : (
              <path
                d="M12 8v5m0 3.5v.01M12 3 2 20h20L12 3Z"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            )}
          </svg>
          <span>{t.message}</span>
          <button type="button" aria-label="Cerrar" onClick={() => dismissToast(t.id)}>
            ×
          </button>
        </div>
      ))}
    </div>
  )
}
