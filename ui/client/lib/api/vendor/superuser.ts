export type TSuperuserOrg = {
  id: string
  name: string
  subdomain: string
  nuon_org_id: string
  is_member?: boolean
}

type TSuperuserSearchResponse = {
  orgs: TSuperuserOrg[]
}

type TSuperuserOrgResponse = {
  org: TSuperuserOrg
}

async function parseError(response: Response, fallback: string): Promise<never> {
  const message = (await response.json().catch(() => null))?.error ?? fallback
  throw new Error(message)
}

export async function searchSuperuserOrgs(query: string): Promise<TSuperuserOrg[]> {
  const response = await fetch(
    `/bff/admin/superuser-api/orgs/search?q=${encodeURIComponent(query)}`,
    { credentials: 'include' }
  )

  if (!response.ok) {
    return parseError(response, 'Failed to search organizations')
  }

  const data = (await response.json()) as TSuperuserSearchResponse
  return data.orgs
}

export async function getSuperuserOrg(orgId: string): Promise<TSuperuserOrg> {
  const response = await fetch(`/bff/admin/superuser-api/orgs/${orgId}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return parseError(response, 'Failed to load organization details')
  }

  const data = (await response.json()) as TSuperuserOrgResponse
  return data.org
}

export async function joinSuperuserOrg(orgId: string): Promise<TSuperuserOrg> {
  const response = await fetch(`/bff/admin/superuser-api/orgs/${orgId}/join`, {
    method: 'POST',
    credentials: 'include',
  })

  if (!response.ok) {
    return parseError(response, 'Failed to join organization')
  }

  const data = (await response.json()) as TSuperuserOrgResponse
  return data.org
}

export async function leaveSuperuserOrg(orgId: string): Promise<TSuperuserOrg> {
  const response = await fetch(`/bff/admin/superuser-api/orgs/${orgId}/leave`, {
    method: 'DELETE',
    credentials: 'include',
  })

  if (!response.ok) {
    return parseError(response, 'Failed to leave organization')
  }

  const data = (await response.json()) as TSuperuserOrgResponse
  return data.org
}
