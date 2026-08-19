export type TVendorMe = {
  id: string
  email: string
  name: string
}

export class VendorAuthFailureError extends Error {
  constructor(message = 'Vendor session is not authenticated') {
    super(message)
    this.name = 'VendorAuthFailureError'
  }
}

export class VendorAuthUnavailableError extends Error {
  constructor(message = 'Vendor auth check is temporarily unavailable') {
    super(message)
    this.name = 'VendorAuthUnavailableError'
  }
}

function isLoginPath(pathname: string): boolean {
  return pathname === '/login' || pathname === '/login/' || pathname === '/admin/login' || pathname === '/admin/login/'
}

function getResponsePath(responseUrl: string): string {
  try {
    return new URL(responseUrl, window.location.origin).pathname
  } catch {
    return ''
  }
}

export async function getMe(): Promise<TVendorMe> {
  const response = await fetch('/admin/profile/me', {
    credentials: 'include',
  })

  const responsePath = getResponsePath(response.url)

  if (response.status === 401 || response.status === 403) {
    throw new VendorAuthFailureError()
  }

  if (response.redirected && isLoginPath(responsePath)) {
    throw new VendorAuthFailureError()
  }

  const contentType = response.headers.get('content-type') ?? ''

  if (response.ok && !contentType.includes('application/json') && isLoginPath(responsePath)) {
    throw new VendorAuthFailureError()
  }

  if (!response.ok) {
    throw new VendorAuthUnavailableError(`Vendor auth request failed with status ${response.status}`)
  }

  if (!contentType.includes('application/json')) {
    throw new VendorAuthUnavailableError('Vendor auth response was not JSON')
  }

  return response.json()
}
