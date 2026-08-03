export const buildVendorOrgUrl = (orgId: string, path = ''): string => {
  const normalizedPath = path.replace(/^\/+/, '')
  const encodedOrgId = encodeURIComponent(orgId)

  return normalizedPath
    ? `/admin/orgs/${encodedOrgId}/${normalizedPath}`
    : `/admin/orgs/${encodedOrgId}/`
}
