export type TAppCatalogStatus = "unpublished" | "coming_soon" | "published";

export type TAppCatalogItem = {
  id: string;
  name: string;
  platform: string;
  logo_light_base64: string;
  logo_dark_base64: string;
  deleted: boolean;
  status: TAppCatalogStatus;
};

export type TAppCatalogPagination = {
  current_page: number;
  has_previous: boolean;
  has_next: boolean;
  previous_page: number;
  next_page: number;
};

type TAppCatalogResponse = {
  apps: TAppCatalogItem[];
  pagination: TAppCatalogPagination;
};

export async function getAppCatalog(
  orgId: string,
  params?: { page?: number },
): Promise<TAppCatalogResponse> {
  const search = new URLSearchParams();
  if (params?.page) search.set("page", String(params.page));

  const response = await fetch(
    `/bff/admin/orgs/${orgId}/apps-api/catalog${search.toString() ? `?${search.toString()}` : ""}`,
    {
      credentials: "include",
      headers: { Accept: "application/json" },
    },
  );

  if (!response.ok) throw response;
  return (await response.json()) as TAppCatalogResponse;
}

export async function updateAppCatalog(
  orgId: string,
  payload: { app_ids: string[]; app_statuses: Record<string, TAppCatalogStatus> },
): Promise<void> {
  const response = await fetch(`/bff/admin/orgs/${orgId}/apps/order`, {
    method: "PUT",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? "Failed to save app catalog settings");
  }
}

export async function forgetDeletedApp(orgId: string, appId: string): Promise<void> {
  const response = await fetch(`/bff/admin/orgs/${orgId}/apps/${appId}/forget`, {
    method: "DELETE",
    credentials: "include",
    headers: { Accept: "application/json" },
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? "Failed to remove app");
  }
}