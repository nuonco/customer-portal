import { Badge } from "@/components/common/Badge";
import { Card } from "@/components/common/Card";
import { SimpleTable } from "@/components/common/SimpleTable";
import { Text } from "@/components/common/Text";
import {
  formatInstallDate,
  formatInstallStatus,
  getInstallStatusBadgeTheme,
} from "@/utils/install-utils";
import { useSelectedInstall } from "./SelectedInstallProvider";

export const CustomerInstallComponentsView = () => {
  const { installDetail } = useSelectedInstall();
  const componentItems = installDetail.components.items ?? [];
  const componentDeploys = installDetail.components.deploys ?? [];

  return (
    <div className="space-y-4">
      <Card className="bg-surface">
        <Text as="h3" role="heading" level={3} variant="h3" weight="stronger">
          Components
        </Text>
        {componentItems.length === 0 ? (
          <Text variant="body" theme="neutral">
            No components found.
          </Text>
        ) : (
          <SimpleTable
            headers={[
              { key: "name", label: "Component name" },
              { key: "type", label: "Type" },
              { key: "status", label: "Status" },
            ]}
          >
            {componentItems.map((component) => (
              <tr key={`${component.id || "-"}-${component.name}-${component.type}`}>
                <td className="px-4 py-3">
                  <Text variant="body" weight="stronger">
                    {component.name}
                  </Text>
                  <Text as="div" variant="subtext" theme="neutral">
                    {component.id || "-"}
                  </Text>
                </td>
                <td className="px-4 py-3">
                  <Text variant="body">{component.type || "-"}</Text>
                </td>
                <td className="px-4 py-3">
                  <Badge theme={getInstallStatusBadgeTheme(component.status)}>
                    {formatInstallStatus(component.status)}
                  </Badge>
                </td>
              </tr>
            ))}
          </SimpleTable>
        )}
      </Card>

      <Card className="bg-surface">
        <Text as="h3" role="heading" level={3} variant="h3" weight="stronger">
          Recent Deploys
        </Text>
        {componentDeploys.length === 0 ? (
          <Text variant="body" theme="neutral">
            No deploys found.
          </Text>
        ) : (
          <SimpleTable
            headers={[
              { key: "id", label: "Deploy" },
              { key: "component", label: "Component" },
              { key: "status", label: "Status" },
              { key: "created", label: "Created" },
            ]}
          >
            {componentDeploys.map((deploy) => (
              <tr key={deploy.id}>
                <td className="px-4 py-3">
                  <Text variant="body">{deploy.id}</Text>
                </td>
                <td className="px-4 py-3">
                  <Text variant="body">{deploy.component_id || "-"}</Text>
                </td>
                <td className="px-4 py-3">
                  <Badge theme={getInstallStatusBadgeTheme(deploy.status)}>
                    {formatInstallStatus(deploy.status)}
                  </Badge>
                </td>
                <td className="px-4 py-3">
                  <Text variant="body">{formatInstallDate(deploy.created_at)}</Text>
                </td>
              </tr>
            ))}
          </SimpleTable>
        )}
      </Card>
    </div>
  );
};
