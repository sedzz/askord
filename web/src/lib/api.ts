// Error para un fallo de la API: si el servidor explicó el motivo (4xx), se muestra tal cual.
export async function failure(res: Response): Promise<Error> {
  if (res.status >= 400 && res.status < 500) {
    const text = (await res.text()).trim()
    if (text) return new Error(text.charAt(0).toUpperCase() + text.slice(1))
  }
  return new Error('Algo salió mal. Inténtalo de nuevo.')
}

export const jsonHeaders = { 'Content-Type': 'application/json' }
