import { beforeEach, expect, mock, test } from 'bun:test'
import { getAccounts } from './get-accounts'

beforeEach(() => {
  const fetchMock = mock(async () => new Response('not found', { status: 404 }))
  Object.defineProperty(globalThis, 'fetch', {
    configurable: true,
    value: fetchMock,
  })
})

test('getAccounts requests JSON from the accounts-api route', async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString()

    expect(url).toBe('/bff/admin/orgs/org-1/accounts-api')
    expect(init?.credentials).toBe('include')
    expect((init?.headers as Record<string, string>).Accept).toBe('application/json')

    return new Response(
      JSON.stringify({ accounts: [], org_id: 'org-1' }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )
  })

  Object.defineProperty(globalThis, 'fetch', {
    configurable: true,
    value: fetchMock,
  })

  const result = await getAccounts('org-1')
  expect(result.org_id).toBe('org-1')
  expect(result.accounts).toEqual([])
})

test('getAccounts appends search query when provided', async () => {
  const fetchMock = mock(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString()
    expect(url).toBe('/bff/admin/orgs/org-1/accounts-api?q=acme')

    return new Response(
      JSON.stringify({
        accounts: [
          { id: 'acc-1', name: 'Acme Corp', member_count: 3, install_count: 2, created_at: '2024-01-01' },
        ],
        org_id: 'org-1',
      }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )
  })

  Object.defineProperty(globalThis, 'fetch', {
    configurable: true,
    value: fetchMock,
  })

  const result = await getAccounts('org-1', { q: 'acme' })
  expect(result.accounts).toHaveLength(1)
  expect(result.accounts[0].name).toBe('Acme Corp')
})

test('getAccounts throws on non-ok response', async () => {
  // fetch mock returns 404 from beforeEach
  await expect(getAccounts('org-1')).rejects.toBeTruthy()
})
