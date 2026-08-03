import { createContext, createElement, type ReactNode } from "react";
import type { TCustomerPortalNavigation } from "@/hooks/use-customer-navigation";

type TCustomerNavigationProviderProps = {
  navigation: TCustomerPortalNavigation;
  children: ReactNode;
};

export const CustomerNavigationContext =
  createContext<TCustomerPortalNavigation | null>(null);

export function CustomerNavigationProvider({
  navigation,
  children,
}: TCustomerNavigationProviderProps) {
  return createElement(
    CustomerNavigationContext.Provider,
    { value: navigation },
    children,
  );
}
