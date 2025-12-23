import type { ReactNode } from 'react'
import { HubProvider, useHubs, useHubPolling } from './HubContext'
import { FormProvider, useForms, useFormPolling } from './FormContext'
import { NodeProvider, useNodes, useNodePolling } from './NodeContext'

export {
  HubProvider,
  useHubs,
  useHubPolling,
  FormProvider,
  useForms,
  useFormPolling,
  NodeProvider,
  useNodes,
  useNodePolling,
}

// Combined provider for convenience
export function LynqProvider({ children }: { children: ReactNode }) {
  return (
    <HubProvider>
      <FormProvider>
        <NodeProvider>{children}</NodeProvider>
      </FormProvider>
    </HubProvider>
  )
}
