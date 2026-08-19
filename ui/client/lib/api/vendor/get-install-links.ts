export type TInstallLinkInstall = {
  id: string
  name: string
  region: string
}

export type TInstallLink = {
  id: string
  org_id: string
  app_id: string
  app_name: string
  name: string
  sha: string
  used: boolean
  created_at: string
  install?: TInstallLinkInstall
}

export type TInstallLinkDetailResponse = {
  link: TInstallLink
  install_url: string
  customer_dashboard_install_url: string
  nuon_dashboard_install_url: string
}

export type TInstallLinkPagination = {
  current_page: number
  total_pages: number
  has_previous: boolean
  has_next: boolean
  previous_page: number
  next_page: number
  total_count: number
  per_page: number
  showing_from: number
  showing_to: number
  current_tab: string
  available_count: number
  used_count: number
}

type TInstallLinksResponse = {
  links: TInstallLink[]
  pagination: TInstallLinkPagination
}

type TOrgApp = {
  id: string
  name: string
}

type TInputField = {
  name: string
  display_name?: string
  description?: string
  type?: string
  required?: boolean
  default?: string
  sensitive?: boolean
  index?: number
}

type TInputGroup = {
  name?: string
  display_name?: string
  description?: string
  app_inputs?: TInputField[]
}

type TInputConfigResponse = {
  platform?: string
  input_config?: {
    input_groups?: TInputGroup[]
  }
}

export async function getInstallLinks(
  orgId: string,
  params?: { tab?: 'available' | 'used'; page?: number }
): Promise<TInstallLinksResponse> {
  const search = new URLSearchParams()
  if (params?.tab) search.set('tab', params.tab)
  if (params?.page) search.set('page', String(params.page))

  const response = await fetch(
    `/admin/orgs/${orgId}/install-links-api${search.toString() ? `?${search.toString()}` : ''}`,
    {
      credentials: 'include',
      headers: { Accept: 'application/json' },
    }
  )

  if (!response.ok) throw response
  return (await response.json()) as TInstallLinksResponse
}

export async function getInstallLinkDetail(
  orgId: string,
  linkId: string
): Promise<TInstallLinkDetailResponse> {
  const response = await fetch(`/admin/orgs/${orgId}/install-links-api/${linkId}`, {
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })

  if (!response.ok) throw response
  return (await response.json()) as TInstallLinkDetailResponse
}

export async function deleteInstallLink(orgId: string, linkId: string): Promise<void> {
  const response = await fetch(`/admin/orgs/${orgId}/install-links-api/${linkId}`, {
    method: 'DELETE',
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })

  if (!response.ok) throw response
}

export async function getOrgApps(orgId: string): Promise<TOrgApp[]> {
  const response = await fetch(`/admin/orgs/${orgId}/apps-api`, {
    credentials: 'include',
  })

  if (!response.ok) throw response
  return (await response.json()) as TOrgApp[]
}

export async function getVendorAppInputConfig(orgId: string, appId: string) {
  const response = await fetch(
    `/admin/orgs/${orgId}/apps-api/${appId}/input-config?filter=vendor`,
    {
      credentials: 'include',
    }
  )

  if (!response.ok) throw response
  return (await response.json()) as TInputConfigResponse
}

export async function createInstallLink(
  orgId: string,
  payload: {
    app_id: string
    app_name: string
    name: string
    inputs: Record<string, string>
  }
): Promise<{ link: TInstallLink }> {
  const response = await fetch(`/admin/orgs/${orgId}/install-links-api`, {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify(payload),
  })

  const body = (await response.json().catch(() => ({}))) as {
    error?: string
    link?: TInstallLink
  }

  if (!response.ok || !body.link) {
    throw new Error(body.error ?? 'Failed to create install link')
  }

  return { link: body.link }
}
