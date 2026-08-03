import { createContext, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  CustomerAuthUnavailableError,
  getCustomerLoginURL,
  getCustomerSessionStatus,
  logoutCustomer,
  type TCustomerSessionStatus,
} from "@/lib/api/customer/get-session";
import { extractSubdomain } from "@/utils/subdomain-utils";

export interface ICustomerAuthContext {
  sessionStatus: TCustomerSessionStatus | undefined;
  isAuthenticated: boolean;
  isLoading: boolean;
  hasError: boolean;
  retry: () => void;
  startLogin: () => Promise<void>;
  logout: () => Promise<void>;
}

export const CustomerAuthContext = createContext<ICustomerAuthContext | undefined>(
  undefined,
);

export function CustomerAuthProvider({ children }: { children: ReactNode }) {
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ["customer", "auth", "session"],
    queryFn: getCustomerSessionStatus,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: true,
    retry: (failureCount, failureError) =>
      !(failureError instanceof CustomerAuthUnavailableError) && failureCount < 2,
    retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 5000),
  });

  const isAuthFailure = !isLoading && !isError && data === "unauthenticated";

  const startLogin = async () => {
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

    const currentSubdomain =
      extractSubdomain(window.location.host, inferredBaseDomain) || fallbackSubdomain;

    const authURL = await getCustomerLoginURL({
      redirect: "/",
      subdomain: currentSubdomain || undefined,
      uiPort: window.location.port || undefined,
    });
    window.location.assign(authURL);
  };

  const logout = async () => {
    await logoutCustomer();
    await refetch();
  };

  return (
    <CustomerAuthContext.Provider
      value={{
        sessionStatus: data,
        isAuthenticated: data === "authenticated",
        isLoading,
        hasError: isError && !isAuthFailure,
        retry: () => {
          void refetch();
        },
        startLogin,
        logout,
      }}
    >
      {children}
    </CustomerAuthContext.Provider>
  );
}
