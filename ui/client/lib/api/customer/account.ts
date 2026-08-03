import { extractSubdomain } from "@/utils/subdomain-utils";

function inferCustomerSubdomain(): string {
  if (typeof window === "undefined") {
    return "";
  }

  const configuredBaseDomain =
    (import.meta.env.VITE_SUBDOMAIN_BASE_DOMAIN as string | undefined) ?? "";
  const fallbackSubdomain =
    (import.meta.env.VITE_CUSTOMER_SUBDOMAIN as string | undefined) ?? "";

  const inferredBaseDomain = (() => {
    if (configuredBaseDomain) {
      return configuredBaseDomain;
    }

    const [hostname, port] = window.location.host.split(":");
    if (!hostname) {
      return window.location.host;
    }

    const labels = hostname.split(".");
    if (labels.length <= 1) {
      return window.location.host;
    }

    const host = labels.slice(1).join(".");
    return port ? `${host}:${port}` : host;
  })();

  return (
    extractSubdomain(window.location.host, inferredBaseDomain) ||
    fallbackSubdomain
  );
}

function withSubdomain(pathname: string): string {
  const query = new URLSearchParams();
  const subdomain = inferCustomerSubdomain();
  if (subdomain) {
    query.set("subdomain", subdomain);
  }

  return `${pathname}${query.toString() ? `?${query.toString()}` : ""}`;
}

async function readError(
  response: Response,
  fallback: string,
): Promise<string> {
  try {
    const data = (await response.json()) as { error?: string };
    return data.error || fallback;
  } catch {
    return fallback;
  }
}

export async function createCustomerAccount(name: string): Promise<void> {
  const response = await fetch(withSubdomain("/bff/portal-api/accounts"), {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    body: JSON.stringify({ name }),
  });

  if (!response.ok) {
    throw new Error(
      await readError(response, "Unable to create a new group right now."),
    );
  }
}

export async function switchCustomerAccount(accountId: string): Promise<void> {
  const response = await fetch(
    withSubdomain("/bff/portal-api/accounts/switch"),
    {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: JSON.stringify({ account_id: accountId }),
    },
  );

  if (!response.ok) {
    throw new Error(
      await readError(response, "Unable to switch groups right now."),
    );
  }
}
