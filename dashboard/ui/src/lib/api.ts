import type {
  LynqHub,
  LynqForm,
  LynqNode,
  ListResponse,
  KubeContext,
  TopologyData,
  KubernetesEvent,
} from '@/types/lynq'

const API_BASE = '/api/v1'

class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function fetchApi<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const response = await fetch(`${API_BASE}${endpoint}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  })

  if (!response.ok) {
    const errorText = await response.text()
    throw new ApiError(response.status, errorText || response.statusText)
  }

  return response.json()
}

// Hub API
export const hubApi = {
  list: (namespace?: string) =>
    fetchApi<ListResponse<LynqHub>>(
      `/hubs${namespace ? `?namespace=${namespace}` : ''}`
    ),

  get: (name: string, namespace?: string) =>
    fetchApi<LynqHub>(
      `/hubs/${name}${namespace ? `?namespace=${namespace}` : ''}`
    ),

  getNodes: (name: string, namespace?: string) =>
    fetchApi<ListResponse<LynqNode>>(
      `/hubs/${name}/nodes${namespace ? `?namespace=${namespace}` : ''}`
    ),
}

// Form Details response type
export interface FormDetailsResponse {
  form: LynqForm
  variables: string[]
}

// Form API
export const formApi = {
  list: (namespace?: string) =>
    fetchApi<ListResponse<LynqForm>>(
      `/forms${namespace ? `?namespace=${namespace}` : ''}`
    ),

  get: (name: string, namespace?: string) =>
    fetchApi<LynqForm>(
      `/forms/${name}${namespace ? `?namespace=${namespace}` : ''}`
    ),

  getDetails: (name: string, namespace?: string) =>
    fetchApi<FormDetailsResponse>(
      `/forms/${name}/details${namespace ? `?namespace=${namespace}` : ''}`
    ),
}

// Node API
export const nodeApi = {
  list: (namespace?: string) =>
    fetchApi<ListResponse<LynqNode>>(
      `/nodes${namespace ? `?namespace=${namespace}` : ''}`
    ),

  get: (name: string, namespace?: string) =>
    fetchApi<LynqNode>(
      `/nodes/${name}${namespace ? `?namespace=${namespace}` : ''}`
    ),

  getResources: (name: string, namespace?: string) =>
    fetchApi<ListResponse<unknown>>(
      `/nodes/${name}/resources${namespace ? `?namespace=${namespace}` : ''}`
    ),
}

// Context API (local mode only)
export const contextApi = {
  list: () => fetchApi<ListResponse<KubeContext>>('/contexts'),

  switch: (contextName: string) =>
    fetchApi<{ success: boolean }>('/contexts/switch', {
      method: 'POST',
      body: JSON.stringify({ context: contextName }),
    }),
}

// Topology API
export const topologyApi = {
  get: (namespace?: string) =>
    fetchApi<TopologyData>(
      `/topology${namespace ? `?namespace=${namespace}` : ''}`
    ),
}

// Resource API (generic K8s resources)
export const resourceApi = {
  get: (kind: string, name: string, namespace?: string) =>
    fetchApi<unknown>(
      `/resources?kind=${encodeURIComponent(kind)}&name=${encodeURIComponent(name)}${namespace ? `&namespace=${encodeURIComponent(namespace)}` : ''}`
    ),
}

// Events API
export const eventsApi = {
  // Generic events endpoint (works for any resource type)
  getEvents: (name: string, namespace?: string) =>
    fetchApi<{ items: KubernetesEvent[] }>(
      `/events?name=${encodeURIComponent(name)}${namespace ? `&namespace=${encodeURIComponent(namespace)}` : ''}`
    ),

  // Polling endpoint for node events (legacy, kept for compatibility)
  getNodeEvents: (name: string, namespace?: string) =>
    fetchApi<{ items: KubernetesEvent[] }>(
      `/nodes/${name}/events${namespace ? `?namespace=${namespace}` : ''}`
    ),
}

// Health API
export const healthApi = {
  check: () => fetch('/healthz').then((r) => r.ok),
  ready: () => fetch('/readyz').then((r) => r.ok),
}

export { ApiError }
