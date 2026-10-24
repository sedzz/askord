import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { ContentSkeleton } from './components/skeletons'
import { routeTree } from './routeTree.gen'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1 } },
})

const router = createRouter({
  routeTree,
  context: { queryClient },
  // Si una navegación tarda (por ejemplo, al descargar una pantalla por primera vez), se
  // muestra un esqueleto en vez de dejar la pantalla anterior congelada.
  defaultPendingMs: 100,
  defaultPendingMinMs: 0,
  defaultPendingComponent: ContentSkeleton,
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
