export type TVendorOrg = {
  id: string
  name: string
  subdomain?: string
}

type TVendorOrgsResponse = {
  orgs: TVendorOrg[]
}

export async function getVendorOrgs(): Promise<TVendorOrg[]> {
  const response = await fetch('/bff/admin/profile/orgs', {
    credentials: 'include',
  })

  if (response.status === 401 || response.status === 403) {
    return []
  }

  if (!response.ok) {
    throw response
  }

  const data = (await response.json()) as TVendorOrgsResponse
  return data.orgs ?? []
}
