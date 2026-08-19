export type TCreateOrgInput = {
  orgId: string
  apiToken: string
  apiUrl?: string
}

export type TCreateOrgResult = {
  id: string
  name: string
  redirect_to?: string
}

export async function createOrg(input: TCreateOrgInput): Promise<TCreateOrgResult> {
  const response = await fetch('/admin/org/create', {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      org_id: input.orgId,
      api_token: input.apiToken,
      api_url: input.apiUrl || undefined,
    }),
  })

  const json = await response.json().catch(() => ({}))

  if (!response.ok) {
    const message = typeof json?.error === 'string' ? json.error : 'Failed to connect org'
    throw new Error(message)
  }

  return json as TCreateOrgResult
}
