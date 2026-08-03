import { useState } from "react";
import { Button } from "@/components/common/Button";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { useCustomerAuth } from "@/hooks/use-customer-auth";

export const HomeView = () => {
  const { logout } = useCustomerAuth();
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [logoutError, setLogoutError] = useState<string | null>(null);

  const handleLogout = async () => {
    setIsLoggingOut(true);
    setLogoutError(null);
    try {
      await logout();
    } catch (error) {
      setLogoutError(
        error instanceof Error ? error.message : "Failed to sign out",
      );
    } finally {
      setIsLoggingOut(false);
    }
  };

  return (
    <main className="min-h-screen p-8 md:p-12">
      <div className="mx-auto flex max-w-5xl flex-col gap-4">
        <Text as="h1" variant="h2" weight="stronger">
          Customer Dashboard
        </Text>
        <Text variant="body" theme="neutral">
          You are signed in.
        </Text>
        <div>
          <Button
            onClick={() => void handleLogout()}
            variant="secondary"
            size="md"
            disabled={isLoggingOut}
          >
            {isLoggingOut ? "Signing out..." : "Sign out"}
          </Button>
        </div>
        {logoutError ? <Message theme="warning">{logoutError}</Message> : null}
      </div>
    </main>
  );
};
