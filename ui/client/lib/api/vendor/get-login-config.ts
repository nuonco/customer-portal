export type TVendorLoginConfig = {
  authUrl: string
  errorMessage: string
  title: string
}

export async function getVendorLoginConfig(
  redirect: string,
  error?: string,
  signal?: AbortSignal
): Promise<TVendorLoginConfig> {
  const requestParams = new URLSearchParams({
    redirect,
  })

  if (error) {
    requestParams.set('error', error)
  }

  const response = await fetch(`/admin/login/config?${requestParams.toString()}`, {
    credentials: 'include',
    signal,
  })

  if (!response.ok) {
    const fallbackMessage = 'Unable to initialize vendor login.'
    try {
      const payload = (await response.json()) as Partial<TVendorLoginConfig>
      throw new Error(payload.errorMessage || fallbackMessage)
    } catch {
      throw new Error(fallbackMessage)
    }
  }

  return (await response.json()) as TVendorLoginConfig
}
