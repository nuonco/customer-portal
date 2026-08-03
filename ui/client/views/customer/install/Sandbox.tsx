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

export const CustomerInstallSandboxView = () => {
  const { installDetail } = useSelectedInstall();
  const sandboxData = installDetail.sandbox;
  const sandboxRuns = sandboxData.recent_runs ?? [];

  return (
    <div className="space-y-4">
      <Card className="bg-surface">
        <Text as="h3" role="heading" level={3} variant="h3" weight="stronger">
          Sandbox Details
        </Text>
        <div className="grid gap-3 md:grid-cols-3">
          <DetailKV
            label="Status"
            value={formatInstallStatus(sandboxData.status)}
          />
          <DetailKV label="Repo" value={sandboxData.repo} />
          <DetailKV label="Branch" value={sandboxData.branch} />
        </div>
        {sandboxData.outputs && Object.keys(sandboxData.outputs).length > 0 ? (
          <div className="mt-4 space-y-3">
            {Object.entries(sandboxData.outputs).map(([key, value]) => (
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
          Recent Sandbox Runs
        </Text>
        {sandboxRuns.length === 0 ? (
          <Text variant="body" theme="neutral">
            No sandbox runs yet.
          </Text>
        ) : (
          <SimpleTable
            headers={[
              { key: "id", label: "Run" },
              { key: "status", label: "Status" },
              { key: "created", label: "Created" },
            ]}
          >
            {sandboxRuns.map((run) => (
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
