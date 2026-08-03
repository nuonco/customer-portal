import { createContext, type ReactNode, useContext } from "react";
import type {
  TCustomerPortalApp,
  TCustomerPortalInstall,
  TCustomerPortalState,
} from "@/lib/api/customer/get-portal-state";

export type TCustomerPortalContext = {
  portalState: TCustomerPortalState;
  currentInstall: TCustomerPortalInstall | null;
  currentApp: TCustomerPortalApp | null;
  activeInstallTab: string | null;
  refetchPortalState: () => Promise<unknown>;
};

const CustomerPortalStateContext = createContext<TCustomerPortalContext | undefined>(
  undefined,
);

export function CustomerPortalStateProvider({
  value,
  children,
}: {
  value: TCustomerPortalContext;
  children: ReactNode;
}) {
  return (
    <CustomerPortalStateContext.Provider value={value}>
      {children}
    </CustomerPortalStateContext.Provider>
  );
}

export function useCustomerPortal(): TCustomerPortalContext {
  const context = useContext(CustomerPortalStateContext);

  if (!context) {
    throw new Error(
      "useCustomerPortal must be used within CustomerPortalStateProvider",
    );
  }

  return context;
}