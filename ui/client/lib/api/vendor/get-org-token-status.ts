export type TVendorOrgTokenStatus = {
  is_valid: boolean
  error_title?: string
  error_message?: string
}

export async function getVendorOrgTokenStatus(orgId: string): Promise<TVendorOrgTokenStatus> {
  const response = await fetch(`/admin/orgs/${orgId}/token-status`, {
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error('Failed to check organization token status')
  }

  return (await response.json()) as TVendorOrgTokenStatus
}
