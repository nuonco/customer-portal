import { Outlet } from "react-router";
import { Button } from "@/components/common/Button";
import { Text } from "@/components/common/Text";
import { VendorAppLayout } from "@/components/vendor/layout/AppLayout";
import { useVendorAuth } from "@/hooks/use-vendor-auth";

export const VendorAuthLayout = () => {
  const { hasError, isAuthenticated, isLoading, retry } = useVendorAuth();

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Text variant="body" theme="neutral">
          Checking your admin session...
        </Text>
      </div>
    );
  }

  if (hasError) {
    return (
      <div className="flex min-h-screen items-center justify-center px-4">
        <div className="flex flex-col items-center gap-6 text-center">
          <Text variant="h2">Unable to connect</Text>
          <Text variant="body" theme="neutral">
            The vendor admin session could not be verified.
          </Text>
          <Button onClick={retry}>Retry</Button>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

  return (
    <VendorAppLayout>
      <Outlet />
    </VendorAppLayout>
  );
};
