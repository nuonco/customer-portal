import { expect, test, mock, beforeEach, afterEach } from 'bun:test'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { VendorAppLayout } from './AppLayout'
import {
  VendorAuthContext,
  type IVendorAuthContext,
} from '@/providers/vendor-auth-provider'
import { SurfacesProvider } from '@/providers/surfaces-provider'

const originalFetch = window.fetch

const baseAuthValue: IVendorAuthContext = {
  hasError: false,
  isAuthenticated: true,
  isLoading: false,
  retry: () => {},
  user: {
    email: 'ada@example.com',
    name: 'Ada Lovelace',
    sub: 'user-1',
  },
}

function renderLayout(pathname = '/admin/orgs') {
  return render(
    <MemoryRouter initialEntries={[pathname]}>
      <VendorAuthContext.Provider value={baseAuthValue}>
        <SurfacesProvider>
          <VendorAppLayout>
            <div>Page content</div>
          </VendorAppLayout>
        </SurfacesProvider>
      </VendorAuthContext.Provider>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  document.cookie = 'org_session=; Max-Age=0; path=/'
  document.cookie = 'sidebar_open=1; path=/'

  const fetchMock = mock(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString()

    if (url.includes('/bff/admin/profile/orgs')) {
      return new Response(JSON.stringify({ orgs: [{ id: 'org-1', name: 'Acme' }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }

    if (url.includes('/bff/admin/orgs/org-1/token-status')) {
      return new Response(JSON.stringify({ is_valid: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }

    return new Response(JSON.stringify({}), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  })

  Object.defineProperty(window, 'fetch', {
    configurable: true,
    value: fetchMock,
  })
})

afterEach(() => {
  cleanup()

  Object.defineProperty(window, 'fetch', {
    configurable: true,
    value: originalFetch,
  })
})

test('disables all sidebar navigation except Documentation when no org is selected', () => {
  const { container } = renderLayout('/admin/orgs')

  const frame = container.querySelector('div.flex.h-screen')
  expect(frame?.className).toContain('bg-app-bg')
  expect(frame?.className).toContain('text-text-primary')

  expect(screen.getByText('Links')).toBeInTheDocument()
  expect(screen.getByText('User Management')).toBeInTheDocument()
  expect(screen.getByText('Settings')).toBeInTheDocument()
  expect(screen.getByText('Resources')).toBeInTheDocument()
  expect(screen.getByText('View Portal')).toBeInTheDocument()
  expect(screen.getByText('View Org')).toBeInTheDocument()
  expect(screen.getByText('Groups')).toBeInTheDocument()

  expect(screen.queryByRole('link', { name: 'View Portal' })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: 'View Org' })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: 'Groups' })).not.toBeInTheDocument()

  const documentationLink = screen.getByRole('link', { name: 'Documentation' })
  expect(documentationLink).toBeInTheDocument()
  expect(documentationLink).toHaveAttribute('href', 'https://docs.nuon.co/guides/customer-portal')
  expect(documentationLink).toHaveAttribute('target', '_blank')

  expect(screen.getByRole('button', { name: /toggle sidebar/i })).toBeInTheDocument()
})

test('keeps org navigation disabled on /admin/orgs even with stale org_session cookie', () => {
  document.cookie = 'org_session=stale-org-id; path=/'

  renderLayout('/admin/orgs')

  expect(screen.queryByRole('link', { name: 'Groups' })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: 'View Org' })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: 'View Portal' })).not.toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Documentation' })).toBeInTheDocument()
})

test('renders org-scoped groups link when org is selected in URL', () => {
  renderLayout('/admin/orgs/org-1/accounts')

  const groupsLink = screen.getByRole('link', { name: 'Groups' })
  expect(groupsLink).toBeInTheDocument()
  expect(groupsLink).toHaveAttribute('href', '/admin/orgs/org-1/accounts')

  const viewOrgLink = screen.getByRole('link', { name: 'View Org' })
  expect(viewOrgLink).toBeInTheDocument()
  expect(viewOrgLink).toHaveAttribute('href', 'https://app.nuon.co/org-1')
  expect(viewOrgLink).toHaveAttribute('target', '_blank')
  expect(viewOrgLink).toHaveAttribute('rel', 'noopener noreferrer')
})

test('collapses sidebar when toggle is clicked', () => {
  const { container } = renderLayout('/admin/orgs')

  const sidebar = container.querySelector('#main-sidebar')
  expect(sidebar).toBeTruthy()
  expect(sidebar?.className).toContain('md:w-[272px]')

  fireEvent.click(screen.getByRole('button', { name: /toggle sidebar/i }))
  expect(sidebar?.className).toContain('md:w-[88px]')
})

test('opens user menu and triggers logout', async () => {
  const assign = mock(() => {})
  const originalLocation = window.location

  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      ...originalLocation,
      assign,
    },
  })

  renderLayout('/admin/orgs')

  fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))

  expect(screen.getByRole('button', { name: /profile/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /keyboard shortcuts/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /superuser/i })).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: /profile/i }))
  await screen.findByText('Profile Settings')

  fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
  fireEvent.click(screen.getByRole('button', { name: /keyboard shortcuts/i }))
  await screen.findByText('Keyboard Shortcuts')

  fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
  fireEvent.click(screen.getByRole('button', { name: /superuser/i }))
  await screen.findByText('Superuser')

  fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
  fireEvent.click(screen.getByRole('button', { name: /log out/i }))

  expect(assign).toHaveBeenCalledWith('/bff/admin/logout')

  Object.defineProperty(window, 'location', {
    configurable: true,
    value: originalLocation,
  })
})

test('shows install-links breadcrumb and install name on install link detail page', async () => {
  const fetchMock = mock(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString()

    if (url.includes('/bff/admin/profile/orgs')) {
      return new Response(JSON.stringify({ orgs: [{ id: 'org-1', name: 'Acme' }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }

    if (url.includes('/bff/admin/orgs/org-1/install-links-api/link-1')) {
      return new Response(
        JSON.stringify({
          link: {
            id: 'link-1',
            org_id: 'org-1',
            app_id: 'app-1',
            app_name: 'Payments API',
            name: 'Install A',
            sha: 'abc123',
            used: false,
            created_at: '2026-01-01T00:00:00Z',
            install: {
              id: 'install-1',
              name: 'Customer Install A',
              region: 'us-east-1',
            },
          },
          install_url: 'https://example.com/install',
          customer_dashboard_install_url: '',
          nuon_dashboard_install_url: '',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )
    }

    return new Response(JSON.stringify({ orgs: [] }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  })

  Object.defineProperty(window, 'fetch', {
    configurable: true,
    value: fetchMock,
  })

  renderLayout('/admin/orgs/org-1/install-links/link-1')

  await waitFor(() => {
    const breadcrumbNav = screen.getByRole('navigation', { name: 'Breadcrumb' })
    expect(within(breadcrumbNav).getByRole('link', { name: 'Install Links' })).toHaveAttribute(
      'href',
      '/admin/orgs/org-1/install-links',
    )
  })

  expect(screen.getByText('Customer Install A')).toBeInTheDocument()

})

test('opens profile panel from URL panel param on initial render', async () => {
  renderLayout('/admin/orgs?panel=profile')

  await screen.findByText('Profile Settings')
})

test('opens keyboard shortcuts modal from URL modal param on initial render', async () => {
  renderLayout('/admin/orgs?modal=keyboard-shortcuts')

  await screen.findByText('Keyboard Shortcuts')
})

test('shows token expired warning when org token check reports expiration', async () => {
  const fetchMock = mock(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString()

    if (url.includes('/bff/admin/profile/orgs')) {
      return new Response(JSON.stringify({ orgs: [{ id: 'org-1', name: 'Acme' }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }

    if (url.includes('/token-status')) {
      return new Response(
        JSON.stringify({
          is_valid: false,
          error_title: 'Token Is Expired',
          error_message: 'Please get a new token from the Nuon dashboard',
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      )
    }

    return new Response(JSON.stringify({}), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  })

  Object.defineProperty(window, 'fetch', {
    configurable: true,
    value: fetchMock,
  })

  renderLayout('/admin/orgs/org-1/accounts')

  const orgConnectionLink = await screen.findByRole('link', { name: 'Org Connection' })
  expect(orgConnectionLink).toHaveAttribute('href', '/admin/orgs/org-1/connection')
})
