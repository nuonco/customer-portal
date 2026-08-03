import { useState, type FormEvent } from "react";
import { Button } from "@/components/common/Button";
import { Card } from "@/components/common/Card";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { Input } from "@/components/common/form/Input";
import { useCustomerPortal } from "@/providers/customer-portal-state-provider";

export const CustomerAccountView = () => {
  const { portalState, refetchPortalState } = useCustomerPortal();
  const [name, setName] = useState(portalState.active_account?.name ?? "");
  const [error, setError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);

    if (!name.trim()) {
      setError("Group name is required.");
      return;
    }

    setIsSaving(true);
    try {
      const response = await fetch("/bff/account", {
        method: "PUT",
        credentials: "include",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify({ name: name.trim() }),
      });

      if (!response.ok) {
        setError("Unable to update the group name right now.");
        return;
      }

      await refetchPortalState();
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(280px,1fr)]">
      <Card className="bg-surface">
        <Text as="h2" role="heading" level={2} variant="h3" weight="stronger">
          Group Settings
        </Text>

        {portalState.active_account ? (
          <form className="space-y-4" onSubmit={(event) => void handleSubmit(event)}>
            <Input
              id="customer-account-name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              labelProps={{ labelText: "Group name" }}
              required
            />
            {error ? <Message theme="warning">{error}</Message> : null}
            <div>
              <Button type="submit" variant="secondary" size="md" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save changes"}
              </Button>
            </div>
          </form>
        ) : (
          <Message theme="info">Create a group from the topbar user menu to manage shared installs.</Message>
        )}
      </Card>

      <Card className="bg-surface">
        <Text as="h3" role="heading" level={3} variant="h3" weight="stronger">
          Account Context
        </Text>
        <div className="space-y-3">
          <div>
            <Text as="div" variant="subtext" weight="stronger" className="uppercase tracking-[0.08em] text-text-muted">
              Current group
            </Text>
            <Text as="div" variant="body">
              {portalState?.active_account?.name ?? "No active group"}
            </Text>
          </div>
          <div>
            <Text as="div" variant="subtext" weight="stronger" className="uppercase tracking-[0.08em] text-text-muted">
              Customer email
            </Text>
            <Text as="div" variant="body">
              {portalState?.user?.email}
            </Text>
          </div>
          <div>
            <Text as="div" variant="subtext" weight="stronger" className="uppercase tracking-[0.08em] text-text-muted">
              Other groups
            </Text>
            {portalState?.other_accounts?.length === 0 ? (
              <Text as="div" variant="body" theme="neutral">
                No additional groups available.
              </Text>
            ) : (
              <div className="space-y-2">
                {portalState?.other_accounts?.map((account) => (
                  <Text as="div" key={account.id} variant="body">
                    {account.name}
                  </Text>
                ))}
              </div>
            )}
          </div>
        </div>
      </Card>
    </div>
  );
};