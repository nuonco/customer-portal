import { afterEach, beforeEach, expect, mock, test } from 'bun:test'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { SurfacesProvider } from '@/providers/surfaces-provider'
import { OrgsView } from './Orgs'

function renderView() {
  return render(
    <MemoryRouter initialEntries={['/admin/orgs']}>
      <SurfacesProvider>
        <OrgsView />
      </SurfacesProvider>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  document.cookie = 'org_session=; path=/; max-age=0'
})

afterEach(() => {
  cleanup()
})

test('redirects to the first org accounts page when orgs are available', async () => {

  const fetchMock = mock(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString()
    if (url === '/admin/profile/orgs') {
      return new Response(
        JSON.stringify({
          orgs: [
            { id: 'org-1', name: 'Alpha' },
            { id: 'org-2', name: 'Bravo' },
          ],
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )
    }

    return new Response('not found', { status: 404 })
  })

  const assign = mock(() => {})
  const originalLocation = window.location

  Object.defineProperty(window, 'fetch', {
    configurable: true,
    value: fetchMock,
  })

  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      ...originalLocation,
      assign,
    },
  })

  renderView()

  await waitFor(() => {
    expect(assign).toHaveBeenCalledWith('/admin/orgs/org-1/accounts')
  })

  Object.defineProperty(window, 'location', {
    configurable: true,
    value: originalLocation,
  })
})

test('shows empty state and connects first org', async () => {
  let createOrgRequestBody: Record<string, unknown> | null = null

  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString()

    if (url === '/admin/profile/orgs') {
      return new Response(
        JSON.stringify({ orgs: [] }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )
    }

    if (url === '/admin/org/create' && init?.method === 'POST') {
      createOrgRequestBody = JSON.parse(String(init.body ?? '{}')) as Record<string, unknown>
      return new Response(
        JSON.stringify({
          id: 'org-new',
          name: 'New Org',
          redirect_to: '/admin/orgs/org-new/install-links',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )
    }

    return new Response('not found', { status: 404 })
  })

  const assign = mock(() => {})
  const originalLocation = window.location

  Object.defineProperty(window, 'fetch', {
    configurable: true,
    value: fetchMock,
  })

  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      ...originalLocation,
      assign,
    },
  })

  renderView()

  await screen.findByText('No Organizations Connected')

  fireEvent.click(screen.getByRole('button', { name: 'Connect Your First Org' }))

  fireEvent.change(screen.getByLabelText('Nuon Org ID'), {
    target: { value: 'org_123' },
  })
  fireEvent.change(screen.getByLabelText('API Token'), {
    target: { value: 'nuon_pat_123' },
  })

  fireEvent.click(screen.getByRole('button', { name: 'Connect' }))

  await waitFor(() => {
    expect(assign).toHaveBeenCalledWith('/admin/orgs/org-new/install-links')
  })

  expect(createOrgRequestBody).toEqual({
    org_id: 'org_123',
    api_token: 'nuon_pat_123',
  })

  Object.defineProperty(window, 'location', {
    configurable: true,
    value: originalLocation,
  })
})
