import { withSubdomain } from "@/lib/runtime-config";
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
  const response = await fetch(withSubdomain("/portal-api/accounts"), {
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
    withSubdomain("/portal-api/accounts/switch"),
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
