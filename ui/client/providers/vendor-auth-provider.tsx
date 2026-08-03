import { createContext, type ReactNode, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { useLocation, useNavigate } from "react-router";
import {
  getMe,
  VendorAuthFailureError,
  type TVendorMe,
} from "@/lib/api/vendor/get-me";
import type { IUser } from "@/types";

export interface IVendorAuthContext {
  user: IUser | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  hasError: boolean;
  retry: () => void;
}

export const VendorAuthContext = createContext<IVendorAuthContext | undefined>(
  undefined,
);

function meToUser(me: TVendorMe): IUser {
  return {
    sub: me.id,
    email: me.email,
    name: me.name,
  };
}

export function VendorAuthProvider({ children }: { children: ReactNode }) {
  const location = useLocation();
  const navigate = useNavigate();

  const {
    data: me,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery({
    queryKey: ["vendor", "auth", "me"],
    queryFn: getMe,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    refetchOnReconnect: true,
    retry: (failureCount, failureError) =>
      !(failureError instanceof VendorAuthFailureError) && failureCount < 2,
    retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 5000),
  });

  const user = me ? meToUser(me) : null;
  const isAuthFailure = error instanceof VendorAuthFailureError;
  const shouldRedirectToLogin = !isLoading && !user && isAuthFailure;

  useEffect(() => {
    if (!shouldRedirectToLogin) {
      return;
    }

    const redirect = encodeURIComponent(location.pathname + location.search);
    navigate(`/admin/login/?redirect=${redirect}`, { replace: true });
  }, [location.pathname, location.search, navigate, shouldRedirectToLogin]);

  return (
    <VendorAuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isLoading,
        hasError: isError && !isAuthFailure,
        retry: () => {
          void refetch();
        },
      }}
    >
      {children}
    </VendorAuthContext.Provider>
  );
}
