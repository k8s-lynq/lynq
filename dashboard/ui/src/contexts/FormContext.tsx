import {
  createContext,
  useContext,
  useReducer,
  useCallback,
  useEffect,
  type ReactNode,
} from 'react'
import type { LynqForm, ListResponse } from '@/types/lynq'
import { formApi } from '@/lib/api'

interface FormState {
  forms: LynqForm[]
  selectedForm: LynqForm | null
  loading: boolean
  error: string | null
  lastUpdated: Date | null
}

type FormAction =
  | { type: 'FETCH_START' }
  | { type: 'FETCH_SUCCESS'; payload: LynqForm[] }
  | { type: 'FETCH_ERROR'; payload: string }
  | { type: 'SELECT_FORM'; payload: LynqForm | null }
  | { type: 'UPDATE_FORM'; payload: LynqForm }

const initialState: FormState = {
  forms: [],
  selectedForm: null,
  loading: false,
  error: null,
  lastUpdated: null,
}

function formReducer(state: FormState, action: FormAction): FormState {
  switch (action.type) {
    case 'FETCH_START':
      return { ...state, loading: true, error: null }
    case 'FETCH_SUCCESS':
      return {
        ...state,
        forms: action.payload,
        loading: false,
        lastUpdated: new Date(),
      }
    case 'FETCH_ERROR':
      return { ...state, loading: false, error: action.payload }
    case 'SELECT_FORM':
      return { ...state, selectedForm: action.payload }
    case 'UPDATE_FORM':
      return {
        ...state,
        forms: state.forms.map((f) =>
          f.metadata.name === action.payload.metadata.name ? action.payload : f
        ),
        selectedForm:
          state.selectedForm?.metadata.name === action.payload.metadata.name
            ? action.payload
            : state.selectedForm,
      }
    default:
      return state
  }
}

interface FormContextValue extends FormState {
  fetchForms: (namespace?: string) => Promise<void>
  selectForm: (form: LynqForm | null) => void
  getForm: (name: string, namespace?: string) => Promise<LynqForm>
}

const FormContext = createContext<FormContextValue | null>(null)

export function FormProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(formReducer, initialState)

  const fetchForms = useCallback(async (namespace?: string) => {
    dispatch({ type: 'FETCH_START' })
    try {
      const response: ListResponse<LynqForm> = await formApi.list(namespace)
      dispatch({ type: 'FETCH_SUCCESS', payload: response.items })
    } catch (err) {
      dispatch({
        type: 'FETCH_ERROR',
        payload: err instanceof Error ? err.message : 'Failed to fetch forms',
      })
    }
  }, [])

  const selectForm = useCallback((form: LynqForm | null) => {
    dispatch({ type: 'SELECT_FORM', payload: form })
  }, [])

  const getForm = useCallback(async (name: string, namespace?: string) => {
    const form = await formApi.get(name, namespace)
    dispatch({ type: 'UPDATE_FORM', payload: form })
    return form
  }, [])

  return (
    <FormContext.Provider value={{ ...state, fetchForms, selectForm, getForm }}>
      {children}
    </FormContext.Provider>
  )
}

export function useForms() {
  const context = useContext(FormContext)
  if (!context) {
    throw new Error('useForms must be used within a FormProvider')
  }
  return context
}

// Polling hook for forms
export function useFormPolling(namespace?: string, interval = 30000) {
  const { fetchForms } = useForms()

  useEffect(() => {
    fetchForms(namespace)
    const timer = setInterval(() => fetchForms(namespace), interval)
    return () => clearInterval(timer)
  }, [fetchForms, namespace, interval])
}
