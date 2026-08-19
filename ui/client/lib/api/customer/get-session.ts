export type TCustomerSessionStatus = "authenticated" | "unauthenticated";

export type TCustomerSessionUser = {
  id: string;
  email: string;
  name: string;
  role: string;
};

type TCustomerSessionResponse = {
  authenticated: boolean;
  user?: TCustomerSessionUser;
};

type TCustomerLoginURLResponse = {
  auth_url: string;
};

type TCustomerLoginURLOptions = {
  redirect?: string;
  subdomain?: string;
};

export class CustomerAuthUnavailableError extends Error {
  constructor(message = "Customer auth check is temporarily unavailable") {
    super(message);
    this.name = "CustomerAuthUnavailableError";
  }
}

export async function getCustomerSessionStatus(): Promise<TCustomerSessionStatus> {
  const response = await fetch("/auth-api/session", {
    credentials: "include",
    headers: { Accept: "application/json" },
  });

  if (response.status === 401 || response.status === 403) {
    return "unauthenticated";
  }

  if (!response.ok) {
    throw new CustomerAuthUnavailableError(
      `Customer auth request failed with status ${response.status}`,
    );
  }

  const data = (await response.json()) as TCustomerSessionResponse;
  return data.authenticated ? "authenticated" : "unauthenticated";
}

export async function getCustomerLoginURL(options: TCustomerLoginURLOptions = {}): Promise<string> {
  const query = new URLSearchParams({ redirect: options.redirect ?? "/" });
  if (options.subdomain) {
    query.set("subdomain", options.subdomain);
  }
  const response = await fetch(`/auth-api/login-url?${query.toString()}`, {
    credentials: "include",
    headers: { Accept: "application/json" },
  });

  if (!response.ok) {
    throw new CustomerAuthUnavailableError(
      `Customer login URL request failed with status ${response.status}`,
    );
  }

  const data = (await response.json()) as TCustomerLoginURLResponse;
  if (!data.auth_url) {
    throw new CustomerAuthUnavailableError("Customer login URL was missing");
  }

  return data.auth_url;
}

export async function logoutCustomer(): Promise<void> {
  const response = await fetch("/auth-api/logout", {
    method: "POST",
    credentials: "include",
    headers: { Accept: "application/json" },
  });

  if (!response.ok) {
    throw new CustomerAuthUnavailableError(
      `Customer logout request failed with status ${response.status}`,
    );
  }
}
