import { useContext } from "react";
import {
  CustomerAuthContext,
  type ICustomerAuthContext,
} from "@/providers/customer-auth-provider";

export function useCustomerAuth(): ICustomerAuthContext {
  const ctx = useContext(CustomerAuthContext);
  if (!ctx) {
    throw new Error("useCustomerAuth must be used within CustomerAuthProvider");
  }
  return ctx;
}
