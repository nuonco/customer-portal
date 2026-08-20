import { Button } from "@/components/common/Button";
import { Card } from "@/components/common/Card";
import { SimpleTable } from "@/components/common/SimpleTable";
import { Text } from "@/components/common/Text";
import { InstallButton } from "@/components/customer/layout/InstallButton";
import { useCustomerPortal } from "@/providers/customer-portal-state-provider";
import { formatInstallDate, formatInstallStatus } from "@/utils/install-utils";

export const CustomerInstallsView = () => {
  const { portalState } = useCustomerPortal();
  const installs = portalState.installs ?? [];

  if (installs.length === 0) {
    return (
      <Card className="bg-surface">
        <Text as="h2" role="heading" level={2} variant="h3" weight="stronger">
          Installs
        </Text>
        <Text variant="body" theme="neutral">
          No installs are visible for your current group yet.
        </Text>
        <InstallButton />
      </Card>
    );
  }

  return (
    <div>
      <div className="flex justify-between gap-4 mb-6">
        <div>
          <Text as="h2" role="heading" level={2} variant="h3" weight="stronger">
            Installs
          </Text>
          <Text variant="body" theme="neutral">
            View and manage all deployed installs here.
          </Text>
        </div>
        <InstallButton showIcon />
      </div>
      <SimpleTable
        headers={[
          { key: "name", label: "Name" },
          { key: "app", label: "App" },
          { key: "status", label: "Status" },
          { key: "created", label: "Created" },
          { key: "actions", label: "Actions", align: "right" },
        ]}
      >
        {installs.map((install) => (
          <tr key={install.id}>
            <td className="px-4 py-3 align-top">
              <Text as="div" variant="body" weight="strong">
                {install.name}
              </Text>
              <Text as="div" variant="subtext" theme="neutral">
                {install.visibility === "account"
                  ? "Shared with your group"
                  : "Private install"}
              </Text>
            </td>
            <td className="px-4 py-3 align-top">
              <Text as="div" variant="body">
                {install.app_name || install.app_id || "Install"}
              </Text>
            </td>
            <td className="px-4 py-3 align-top">
              <Text as="div" variant="body">
                {formatInstallStatus(install.status)}
              </Text>
            </td>
            <td className="px-4 py-3 align-top">
              <Text as="div" variant="body" theme="neutral">
                {formatInstallDate(install.created_at)}
              </Text>
            </td>
            <td className="px-4 py-3 align-top text-right">
              <Button
                href={`/installs/${install.id}/overview`}
                variant="secondary"
                size="sm"
              >
                View
              </Button>
            </td>
          </tr>
        ))}
      </SimpleTable>
    </div>
  );
};
