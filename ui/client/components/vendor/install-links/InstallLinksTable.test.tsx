import { describe, expect, mock, test } from 'bun:test'
import { fireEvent, render, screen } from '@testing-library/react'
import { InstallLinksTable } from './InstallLinksTable'

describe('InstallLinksTable', () => {
  test('navigates when a row is clicked', () => {
    const navigateTo = mock(() => {})

    render(
      <InstallLinksTable
        orgId="org-1"
        links={[
          {
            id: 'link-1',
            org_id: 'org-1',
            app_id: 'app-1',
            app_name: 'Payments API',
            name: 'payments-link',
            sha: 'abc123',
            used: false,
            created_at: '2026-01-01T00:00:00Z',
          },
        ]}
        pagination={{
          current_page: 1,
          total_pages: 1,
          has_previous: false,
          has_next: false,
          previous_page: 1,
          next_page: 1,
          total_count: 1,
          per_page: 20,
          showing_from: 1,
          showing_to: 1,
          current_tab: 'available',
          available_count: 1,
          used_count: 0,
        }}
        activeTab="available"
        onTabChange={() => {}}
        onPageChange={() => {}}
        onCreateLink={() => {}}
        navigateTo={navigateTo}
      />,
    )

    fireEvent.click(screen.getByTestId('install-link-row-link-1'))

    expect(navigateTo).toHaveBeenCalledTimes(1)
    expect(navigateTo).toHaveBeenCalledWith('/admin/orgs/org-1/install-links/link-1')
  })

  test('navigates when Enter is pressed on a focused row', () => {
    const navigateTo = mock(() => {})

    render(
      <InstallLinksTable
        orgId="org-1"
        links={[
          {
            id: 'link-2',
            org_id: 'org-1',
            app_id: 'app-2',
            app_name: 'CRM',
            name: 'crm-link',
            sha: 'def456',
            used: true,
            created_at: '2026-01-02T00:00:00Z',
            install: {
              id: 'install-1',
              name: 'CRM East',
              region: 'us-east-1',
            },
          },
        ]}
        pagination={{
          current_page: 1,
          total_pages: 1,
          has_previous: false,
          has_next: false,
          previous_page: 1,
          next_page: 1,
          total_count: 1,
          per_page: 20,
          showing_from: 1,
          showing_to: 1,
          current_tab: 'used',
          available_count: 0,
          used_count: 1,
        }}
        activeTab="used"
        onTabChange={() => {}}
        onPageChange={() => {}}
        onCreateLink={() => {}}
        navigateTo={navigateTo}
      />,
    )

    const row = screen.getByTestId('install-link-row-link-2')
    row.focus()
    fireEvent.keyDown(row, { key: 'Enter' })

    expect(navigateTo).toHaveBeenCalledTimes(1)
    expect(navigateTo).toHaveBeenCalledWith('/admin/orgs/org-1/install-links/link-2')
  })
})
