import { useCallback, useEffect, useMemo, useState } from 'react'
import { useLocation } from 'react-router'
import { getVendorLoginConfig } from '@/lib/api/vendor/get-login-config'

type VendorLoginConfigState = {
  authUrl: string
  errorMessage: string
  isLoading: boolean
  redirectPath: string
  title: string
}

const initialState: VendorLoginConfigState = {
  authUrl: '',
  errorMessage: '',
  isLoading: true,
  redirectPath: '/admin/orgs',
  title: 'Customer Portal',
}

export function normalizeVendorRedirectPath(rawRedirect: string | null): string {
  if (!rawRedirect || !rawRedirect.startsWith('/') || rawRedirect.includes('//')) {
    return '/admin/orgs'
  }

  return rawRedirect
}

function sanitizeUIPort(port: string): string {
  const trimmed = port.trim()
  return /^\d{2,5}$/.test(trimmed) ? trimmed : ''
}

export function buildVendorLoginStateRedirect(redirectPath: string): string {
  if (typeof window === 'undefined') {
    return redirectPath
  }

  const hostname = window.location.hostname
  if (hostname !== 'localhost' && hostname !== '127.0.0.1') {
    return redirectPath
  }

  const uiPort = sanitizeUIPort(window.location.port)
  if (!uiPort || uiPort === '8080') {
    return redirectPath
  }

  const params = new URLSearchParams({
    ui_port: uiPort,
    next: redirectPath,
  })

  return `/auth-api/post-login?${params.toString()}`
}

export function useVendorLoginConfig() {
  const location = useLocation()
  const [attempt, setAttempt] = useState(0)
  const [state, setState] = useState<VendorLoginConfigState>(initialState)

  const redirectPath = useMemo(() => {
    const params = new URLSearchParams(location.search)
    return normalizeVendorRedirectPath(params.get('redirect'))
  }, [location.search])

  const stateRedirectPath = useMemo(
    () => buildVendorLoginStateRedirect(redirectPath),
    [redirectPath],
  )

  const error = useMemo(() => {
    const params = new URLSearchParams(location.search)
    return params.get('error') ?? ''
  }, [location.search])

  const retry = useCallback(() => {
    setAttempt((prev) => prev + 1)
  }, [])

  useEffect(() => {
    let isUnmounted = false
    const controller = new AbortController()

    setState(initialState)

    getVendorLoginConfig(stateRedirectPath, error || undefined, controller.signal)
      .then((parsed) => {
        if (isUnmounted) {
          return
        }

        setState({
          authUrl: parsed.authUrl,
          errorMessage: parsed.errorMessage,
          isLoading: false,
          redirectPath,
          title: parsed.title,
        })
      })
      .catch(() => {
        if (isUnmounted || controller.signal.aborted) {
          return
        }

        setState({
          authUrl: '',
          errorMessage: 'Unable to initialize vendor login.',
          isLoading: false,
          redirectPath,
          title: initialState.title,
        })
      })

    return () => {
      isUnmounted = true
      controller.abort()
    }
  }, [attempt, error, redirectPath, stateRedirectPath])

  return {
    ...state,
    retry,
  }
}