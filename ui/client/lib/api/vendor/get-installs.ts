import type { TCloudPlatform } from '@/types'

export type TAdminInstall = {
  ID: string
  NuonInstallID: string
  NuonOrgID: string
  Name: string
  CustomerID: string
  CustomerEmail: string
  CustomerName: string
  AppID: string
  AppName: string
  Platform: TCloudPlatform
  Status: string
  InstallLinkID: string
  CreatedAt: string
  APIDeleted: boolean
}

type TInstallsResponse = {
  installs: TAdminInstall[]
  org_id: string
  nuon_org_id: string
}

export type TSearchResult = {
  InstallID: string
  InstallName: string
  AppID: string
  AppName: string
  Status: string
  Region: string
}

type TSearchResponse = {
  results: TSearchResult[] | null
}

export async function getInstalls(
  orgId: string,
  filters?: { email?: string; app?: string; platform?: string }
): Promise<TInstallsResponse> {
  const params = new URLSearchParams()
  if (filters?.email) params.set('email', filters.email)
  if (filters?.app) params.set('app', filters.app)
  if (filters?.platform) params.set('platform', filters.platform)

  const qs = params.toString()
  const url = `/bff/admin/orgs/${orgId}/installs-api${qs ? `?${qs}` : ''}`

  const response = await fetch(url, {
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })

  if (!response.ok) throw response

  return (await response.json()) as TInstallsResponse
}

export async function searchNuonInstalls(
  orgId: string,
  q: string
): Promise<TSearchResult[]> {
  const response = await fetch(
    `/bff/admin/orgs/${orgId}/installs-api/search-nuon?q=${encodeURIComponent(q)}`,
    {
      credentials: 'include',
      headers: { Accept: 'application/json' },
    }
  )

  if (!response.ok) throw response

  const data = (await response.json()) as TSearchResponse
  return data.results ?? []
}

export async function importInstall(
  orgId: string,
  payload: { nuon_install_id: string; app_id: string; customer_email: string }
): Promise<void> {
  const body = new URLSearchParams(payload)
  const response = await fetch(`/bff/admin/orgs/${orgId}/installs-api/import`, {
    method: 'POST',
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/x-www-form-urlencoded',
    },
    body: body.toString(),
  })

  if (!response.ok) {
    const text = await response.text()
    throw new Error(text || 'Import failed')
  }
}

export async function forgetInstall(
  orgId: string,
  installId: string
): Promise<void> {
  const response = await fetch(
    `/bff/admin/orgs/${orgId}/installs-api/${installId}/forget`,
    {
      method: 'POST',
      credentials: 'include',
      headers: { Accept: 'application/json' },
    }
  )

  if (!response.ok) {
    const data = (await response.json().catch(() => ({}))) as { error?: string }
    throw new Error(data.error ?? 'Failed to forget install')
  }
}
