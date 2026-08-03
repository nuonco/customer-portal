import { describe, expect, test } from 'bun:test'
import { buildVendorOrgUrl } from './vendor-url-utils'

describe('vendor-url-utils', () => {
  describe('buildVendorOrgUrl', () => {
    test('returns base org path with trailing slash', () => {
      expect(buildVendorOrgUrl('org-1')).toBe('/admin/orgs/org-1/')
    })

    test('builds an org-scoped path', () => {
      expect(buildVendorOrgUrl('org-1', 'connection')).toBe('/admin/orgs/org-1/connection')
    })

    test('normalizes leading slash in path', () => {
      expect(buildVendorOrgUrl('org-1', '/connection')).toBe('/admin/orgs/org-1/connection')
    })

    test('encodes org id safely', () => {
      expect(buildVendorOrgUrl('org id', 'connection')).toBe('/admin/orgs/org%20id/connection')
    })
  })
})
