import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from '@/components/theme-provider'
import { TooltipProvider } from '@/components/ui/tooltip'
import { LynqProvider } from '@/contexts'
import { Layout } from '@/components/Layout'
import {
  Overview,
  Topology,
  Hubs,
  HubDetail,
  Forms,
  FormDetail,
  Nodes,
  NodeDetail,
} from '@/pages'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 1000, // 30 seconds
      refetchInterval: 30 * 1000, // 30 seconds
    },
  },
})

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider defaultTheme="dark">
        <TooltipProvider delayDuration={300}>
          <LynqProvider>
            <BrowserRouter>
              <Routes>
                <Route path="/" element={<Layout />}>
                  <Route index element={<Overview />} />
                  <Route path="topology" element={<Topology />} />
                  <Route path="hubs" element={<Hubs />} />
                  <Route path="hubs/:name" element={<HubDetail />} />
                  <Route path="forms" element={<Forms />} />
                  <Route path="forms/:name" element={<FormDetail />} />
                  <Route path="nodes" element={<Nodes />} />
                  <Route path="nodes/:name" element={<NodeDetail />} />
                </Route>
              </Routes>
            </BrowserRouter>
          </LynqProvider>
        </TooltipProvider>
      </ThemeProvider>
    </QueryClientProvider>
  )
}

export default App
