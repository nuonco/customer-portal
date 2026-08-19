import { inferCustomerSubdomain } from "@/lib/runtime-config";
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
    const currentSubdomain = inferCustomerSubdomain();

    const authURL = await getCustomerLoginURL({
      redirect: "/",
      subdomain: currentSubdomain || undefined,
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
