import { Card } from "@/components/common/Card";
import { SimpleTable } from "@/components/common/SimpleTable";
import { Text } from "@/components/common/Text";
import { EmptyTabState } from "@/components/customer/install/EmptyTabState";
import { useSelectedInstall } from "./SelectedInstallProvider";

export const CustomerInstallRolesView = () => {
  const { installDetail } = useSelectedInstall();
  const roleItems = installDetail.roles.roles ?? [];

  if (roleItems.length === 0) {
    return <EmptyTabState message="No IAM roles were found for this install yet." />;
  }

  return (
    <Card className="bg-surface">
      <Text as="h3" role="heading" level={3} variant="h3" weight="stronger">
        Roles
      </Text>
      <SimpleTable
        headers={[
          { key: "name", label: "Role" },
          { key: "type", label: "Type" },
          { key: "arn", label: "ARN" },
        ]}
      >
        {roleItems.map((role) => (
          <tr key={`${role.name}-${role.arn}`}>
            <td className="px-4 py-3">
              <Text variant="body">{role.name}</Text>
            </td>
            <td className="px-4 py-3">
              <Text variant="body">{role.type}</Text>
            </td>
            <td className="px-4 py-3">
              <Text variant="body">{role.arn}</Text>
            </td>
          </tr>
        ))}
      </SimpleTable>
    </Card>
  );
};
