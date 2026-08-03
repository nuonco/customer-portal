import { useContext } from "react";
import {
  VendorAuthContext,
  type IVendorAuthContext,
} from "@/providers/vendor-auth-provider";

export function useVendorAuth(): IVendorAuthContext {
  const ctx = useContext(VendorAuthContext);
  if (!ctx) {
    throw new Error("useVendorAuth must be used within VendorAuthProvider");
  }
  return ctx;
}
