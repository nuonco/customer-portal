import { Badge } from "@/components/common/Badge";
import { Card } from "@/components/common/Card";
import { CodeBlock } from "@/components/common/CodeBlock";
import { SimpleTable } from "@/components/common/SimpleTable";
import { Text } from "@/components/common/Text";
import { DetailKV } from "@/components/customer/install/DetailKV";
import {
  formatInstallDate,
  formatInstallStatus,
  getInstallStatusBadgeTheme,
} from "@/utils/install-utils";
import { useSelectedInstall } from "./SelectedInstallProvider";

export const CustomerInstallStackView = () => {
  const { installDetail } = useSelectedInstall();
  const stackData = installDetail.stack;
  const stackRuns = stackData.recent_runs ?? [];

  return (
    <div className="space-y-4">
      <Card className="bg-surface">
        <Text as="h3" role="heading" level={3} variant="h3" weight="stronger">
          Stack Details
        </Text>
        <div className="grid gap-3 md:grid-cols-3">
          <DetailKV label="Status" value={formatInstallStatus(stackData.status)} />
          <DetailKV label="Region" value={stackData.region} />
          <DetailKV label="Account" value={stackData.account_id} />
        </div>
        {stackData.outputs && Object.keys(stackData.outputs).length > 0 ? (
          <div className="mt-4 space-y-3">
            {Object.entries(stackData.outputs).map(([key, value]) => (
              <div key={key}>
                <Text
                  as="div"
                  variant="subtext"
                  weight="stronger"
                  className="uppercase tracking-[0.08em] text-text-muted"
                >
                  {key}
                </Text>
                <CodeBlock language="json" showCopy>
                  {value}
                </CodeBlock>
              </div>
            ))}
          </div>
        ) : null}
      </Card>

      <Card className="bg-surface">
        <Text as="h3" role="heading" level={3} variant="h3" weight="stronger">
          Recent Stack Runs
        </Text>
        {stackRuns.length === 0 ? (
          <Text variant="body" theme="neutral">
            No stack runs yet.
          </Text>
        ) : (
          <SimpleTable
            headers={[
              { key: "id", label: "Run" },
              { key: "status", label: "Status" },
              { key: "version", label: "Version Status" },
              { key: "created", label: "Created" },
            ]}
          >
            {stackRuns.map((run) => (
              <tr key={run.id}>
                <td className="px-4 py-3">
                  <Text variant="body">{run.id}</Text>
                </td>
                <td className="px-4 py-3">
                  <Badge theme={getInstallStatusBadgeTheme(run.status)}>
                    {formatInstallStatus(run.status)}
                  </Badge>
                </td>
                <td className="px-4 py-3">
                  <Text variant="body">{run.version_status || "-"}</Text>
                </td>
                <td className="px-4 py-3">
                  <Text variant="body">{formatInstallDate(run.created_at)}</Text>
                </td>
              </tr>
            ))}
          </SimpleTable>
        )}
      </Card>
    </div>
  );
};
