import { beforeEach, expect, mock, test } from 'bun:test'
import {
  createInstallLink,
  deleteInstallLink,
  getInstallLinkDetail,
  getInstallLinks,
} from './get-install-links'

beforeEach(() => {
  const fetchMock = mock(async () => new Response('not found', { status: 404 }))
  Object.defineProperty(globalThis, 'fetch', {
    configurable: true,
    value: fetchMock,
  })
})

test('getInstallLinks requests JSON payload from backend route', async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString()

    expect(url).toBe('/bff/admin/orgs/org-1/install-links-api?tab=available&page=2')
    expect(init?.credentials).toBe('include')
    expect((init?.headers as Record<string, string>).Accept).toBe('application/json')

    return new Response(
      JSON.stringify({
        links: [],
        pagination: {
          current_page: 2,
          total_pages: 2,
          has_previous: true,
          has_next: false,
          previous_page: 1,
          next_page: 2,
          total_count: 11,
          per_page: 10,
          showing_from: 11,
          showing_to: 11,
          current_tab: 'available',
          available_count: 11,
          used_count: 0,
        },
      }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )
  })

  Object.defineProperty(globalThis, 'fetch', {
    configurable: true,
    value: fetchMock,
  })

  const result = await getInstallLinks('org-1', { tab: 'available', page: 2 })
  expect(result.pagination.current_page).toBe(2)
})

test('createInstallLink sends expected payload and returns link id', async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString()

    expect(url).toBe('/bff/admin/orgs/org-1/install-links-api')
    expect(init?.method).toBe('POST')
    expect((init?.headers as Record<string, string>)['Content-Type']).toBe('application/json')

    const body = JSON.parse(String(init?.body)) as {
      app_id: string
      app_name: string
      name: string
      inputs: Record<string, string>
    }

    expect(body.app_id).toBe('app-1')
    expect(body.name).toBe('customer-acme-prod')
    expect(body.inputs.region).toBe('us-east-1')

    return new Response(
      JSON.stringify({
        link: {
          id: 'ink_123',
          app_id: 'app-1',
          app_name: 'Acme App',
          name: 'customer-acme-prod',
          used: false,
          created_at: '2026-06-16T00:00:00Z',
        },
      }),
      { status: 201, headers: { 'Content-Type': 'application/json' } },
    )
  })

  Object.defineProperty(globalThis, 'fetch', {
    configurable: true,
    value: fetchMock,
  })

  const result = await createInstallLink('org-1', {
    app_id: 'app-1',
    app_name: 'Acme App',
    name: 'customer-acme-prod',
    inputs: { region: 'us-east-1' },
  })

  expect(result.link.id).toBe('ink_123')
})

test('getInstallLinkDetail requests JSON payload from detail endpoint', async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString()

    expect(url).toBe('/bff/admin/orgs/org-1/install-links-api/link-1')
    expect(init?.credentials).toBe('include')
    expect((init?.headers as Record<string, string>).Accept).toBe('application/json')

    return new Response(
      JSON.stringify({
        link: {
          id: 'link-1',
          org_id: 'org-1',
          app_id: 'app-1',
          app_name: 'Acme App',
          name: 'customer-acme-prod',
          sha: 'sha-1',
          used: false,
          created_at: '2026-06-16T00:00:00Z',
        },
        install_url: 'https://acme.example.com/install-link?sha=sha-1',
        customer_dashboard_install_url: '',
        nuon_dashboard_install_url: '',
      }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )
  })

  Object.defineProperty(globalThis, 'fetch', {
    configurable: true,
    value: fetchMock,
  })

  const result = await getInstallLinkDetail('org-1', 'link-1')
  expect(result.link.id).toBe('link-1')
  expect(result.install_url).toContain('sha-1')
})

test('deleteInstallLink sends delete request to detail endpoint', async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString()

    expect(url).toBe('/bff/admin/orgs/org-1/install-links-api/link-1')
    expect(init?.method).toBe('DELETE')
    expect(init?.credentials).toBe('include')

    return new Response(JSON.stringify({ message: 'ok' }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  })

  Object.defineProperty(globalThis, 'fetch', {
    configurable: true,
    value: fetchMock,
  })

  await deleteInstallLink('org-1', 'link-1')
  expect(fetchMock).toHaveBeenCalledTimes(1)
})
