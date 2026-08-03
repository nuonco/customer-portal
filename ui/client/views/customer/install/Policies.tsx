import { Badge } from "@/components/common/Badge";
import { Card } from "@/components/common/Card";
import { SimpleTable } from "@/components/common/SimpleTable";
import { Text } from "@/components/common/Text";
import { useSelectedInstall } from "./SelectedInstallProvider";

export const CustomerInstallPoliciesView = () => {
  const { installDetail } = useSelectedInstall();
  const policiesData = installDetail.policies;
  const policyItems = policiesData.items ?? [];

  return (
    <div className="space-y-4">
      <Card className="bg-surface">
        <Text as="h3" role="heading" level={3} variant="h3" weight="stronger">
          Policy Totals
        </Text>
        <div className="flex flex-wrap gap-2">
          <Badge theme="success">Pass: {policiesData.totals?.pass ?? 0}</Badge>
          <Badge theme="warn">Warn: {policiesData.totals?.warn ?? 0}</Badge>
          <Badge theme="error">Deny: {policiesData.totals?.deny ?? 0}</Badge>
        </div>
      </Card>

      <Card className="bg-surface">
        <Text as="h3" role="heading" level={3} variant="h3" weight="stronger">
          Policies
        </Text>
        {policyItems.length === 0 ? (
          <Text variant="body" theme="neutral">
            No policies configured.
          </Text>
        ) : (
          <SimpleTable
            headers={[
              { key: "name", label: "Name" },
              { key: "type", label: "Type" },
              { key: "engine", label: "Engine" },
            ]}
          >
            {policyItems.map((policy) => (
              <tr key={policy.name}>
                <td className="px-4 py-3">
                  <Text variant="body">{policy.name}</Text>
                </td>
                <td className="px-4 py-3">
                  <Text variant="body">{policy.type || "-"}</Text>
                </td>
                <td className="px-4 py-3">
                  <Text variant="body">{policy.engine || "-"}</Text>
                </td>
              </tr>
            ))}
          </SimpleTable>
        )}
      </Card>
    </div>
  );
};
