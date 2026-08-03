export type TAccount = {
  id: string
  name: string
  member_count: number
  install_count: number
  created_at: string
}

type TAccountsResponse = {
  accounts: TAccount[]
  org_id: string
}

export async function getAccounts(
  orgId: string,
  filters?: { q?: string }
): Promise<TAccountsResponse> {
  const params = new URLSearchParams()
  if (filters?.q) params.set('q', filters.q)

  const qs = params.toString()
  const url = `/bff/admin/orgs/${orgId}/accounts-api${qs ? `?${qs}` : ''}`

  const response = await fetch(url, {
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })

  if (!response.ok) throw response

  return (await response.json()) as TAccountsResponse
}
