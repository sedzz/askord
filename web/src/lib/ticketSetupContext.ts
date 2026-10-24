import { createContext, useContext } from 'react'
import type { TicketSetup } from './useTicketSetup'

// El layout del servidor crea el estado de la configuración una sola vez y lo comparten
// el tour y las páginas hijas (así lo que hace el tour se ve de inmediato en Configuración).
export const TicketSetupContext = createContext<TicketSetup | null>(null)

export function useSetup(): TicketSetup {
  const setup = useContext(TicketSetupContext)
  if (!setup) throw new Error('useSetup se usa dentro del layout de un servidor')
  return setup
}
