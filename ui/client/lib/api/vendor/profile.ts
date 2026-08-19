export async function updateVendorProfile(name: string): Promise<void> {
  const response = await fetch('/admin/profile', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    credentials: 'include',
    body: JSON.stringify({ name }),
  })

  if (!response.ok) {
    const message =
      (await response.json().catch(() => null))?.error ?? 'Failed to update profile'
    throw new Error(message)
  }
}
