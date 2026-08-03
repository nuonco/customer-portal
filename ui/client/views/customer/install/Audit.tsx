import { Badge } from "@/components/common/Badge";
import { Card } from "@/components/common/Card";
import { Duration } from "@/components/common/Duration";
import { SimpleTable } from "@/components/common/SimpleTable";
import { Tabs } from "@/components/common/Tabs";
import { Text } from "@/components/common/Text";
import {
  formatInstallDate,
  formatInstallStatus,
  getInstallStatusBadgeTheme,
} from "@/utils/install-utils";
import type {
  TCustomerInstallDetail,
  TCustomerInstallWorkflow,
} from "@/lib/api/customer/get-install-detail";
import { useSelectedInstall } from "./SelectedInstallProvider";

function formatWorkflowCreatedAt(timestamp: string): string {
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) {
    return "Unknown";
  }

  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function EmptyAuditTableState({ message }: { message: string }) {
  return (
    <Text variant="body" theme="neutral">
      {message}
    </Text>
  );
}

function WorkflowAuditTable({
  workflows,
  emptyMessage,
}: {
  workflows: TCustomerInstallWorkflow[];
  emptyMessage: string;
}) {
  if (workflows.length === 0) {
    return <EmptyAuditTableState message={emptyMessage} />;
  }

  return (
    <SimpleTable
      headers={[
        { key: "name", label: "Workflow" },
        { key: "status", label: "Status" },
        { key: "created", label: "Created" },
        { key: "duration", label: "Duration" },
      ]}
    >
      {workflows.map((workflow) => (
        <tr key={workflow.id}>
          <td className="px-4 py-3">
            <Text variant="body">{workflow.name}</Text>
          </td>
          <td className="px-4 py-3">
            <Badge theme={getInstallStatusBadgeTheme(workflow.status)}>
              {formatInstallStatus(workflow.status)}
            </Badge>
          </td>
          <td className="px-4 py-3">
            <Text variant="body">
              {formatWorkflowCreatedAt(workflow.created_at)}
            </Text>
          </td>
          <td className="px-4 py-3">
            <Duration
              variant="body"
              beginTime={workflow.created_at}
              endTime={workflow.finished_at}
              preserveSeconds
            />
          </td>
        </tr>
      ))}
    </SimpleTable>
  );
}

function StackRunAuditTable({
  stackRuns,
}: {
  stackRuns: TCustomerInstallDetail["stack"]["recent_runs"];
}) {
  if (stackRuns.length === 0) {
    return <EmptyAuditTableState message="No stack runs yet." />;
  }

  return (
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
  );
}

function SandboxRunAuditTable({
  sandboxRuns,
}: {
  sandboxRuns: TCustomerInstallDetail["sandbox"]["recent_runs"];
}) {
  if (sandboxRuns.length === 0) {
    return <EmptyAuditTableState message="No sandbox runs yet." />;
  }

  return (
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
  );
}

function ComponentDeployAuditTable({
  deploys,
}: {
  deploys: TCustomerInstallDetail["components"]["deploys"];
}) {
  if (deploys.length === 0) {
    return <EmptyAuditTableState message="No deploys found." />;
  }

  return (
    <SimpleTable
      headers={[
        { key: "id", label: "Deploy" },
        { key: "component", label: "Component" },
        { key: "status", label: "Status" },
        { key: "created", label: "Created" },
      ]}
    >
      {deploys.map((deploy) => (
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
  );
}

export const CustomerInstallAuditView = () => {
  const { installDetail } = useSelectedInstall();
  const auditWorkflows = installDetail.audit.workflows ?? [];
  const actionWorkflows = installDetail.audit.action_workflows ?? [];
  const stackRuns = installDetail.stack.recent_runs ?? [];
  const sandboxRuns = installDetail.sandbox.recent_runs ?? [];
  const componentDeploys = installDetail.components.deploys ?? [];

  return (
    <Card className="bg-surface">
      <Text as="h3" variant="h3" weight="stronger">
        Audit Log
      </Text>
      <Tabs
        initActiveTab="workflows"
        className="mt-4"
        tabs={{
          workflows: (
            <div className="mt-6">
              <WorkflowAuditTable
                workflows={auditWorkflows}
                emptyMessage="No workflows recorded yet."
              />
            </div>
          ),
          stack: (
            <div className="mt-4">
              <StackRunAuditTable stackRuns={stackRuns} />
            </div>
          ),
          sandbox: (
            <div className="mt-4">
              <SandboxRunAuditTable sandboxRuns={sandboxRuns} />
            </div>
          ),
          components: (
            <div className="mt-4">
              <ComponentDeployAuditTable deploys={componentDeploys} />
            </div>
          ),
          actions: (
            <div className="mt-4">
              <WorkflowAuditTable
                workflows={actionWorkflows}
                emptyMessage="No action workflows recorded yet."
              />
            </div>
          ),
        }}
      />
    </Card>
  );
};
