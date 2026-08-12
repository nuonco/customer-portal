import { Button } from "@/components/common/Button";
import { Card } from "@/components/common/Card";
import { Loading } from "@/components/common/Loading";
import { Status } from "@/components/common/Status";
import { Text } from "@/components/common/Text";
import type {
  TCustomerWizardGroup,
  TCustomerWizardState,
} from "@/lib/api/customer/install-wizard";
import { statusTitle } from "../wizard-utils";

export type TWorkflowActionHandlers = {
  actionPending: boolean;
  onApprove: (group: TCustomerWizardGroup) => Promise<void>;
  onRetry: (group: TCustomerWizardGroup) => Promise<void>;
};

export function WaitingForWorkflowDetailsCard() {
  return (
    <Card className="bg-surface">
      <div className="flex items-center gap-3">
        <Loading />
        <div>
          <Text as="div" variant="body" weight="stronger">
            Waiting for workflow details
          </Text>
          <Text as="div" variant="subtext" theme="neutral">
            The provisioning workflow is starting. This view refreshes
            automatically.
          </Text>
        </div>
      </div>
    </Card>
  );
}

export function WorkflowStepsList({
  workflow,
  onRetry,
  actionPending,
  onApprove,
  isReadOnly,
}: {
  workflow: TCustomerWizardState["workflow"];
  isReadOnly?: boolean;
} & TWorkflowActionHandlers) {
  const steps = workflow.groups.flatMap((group) =>
    group.steps.map((step) => ({ ...step, group: group })),
  );

  return (
    <ol className="flex flex-col border rounded-md divide-y">
      {steps.map((step) => (
        <li
          key={step.id}
          className="flex items-center justify-between gap-4 px-4 py-3"
        >
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              <Status status={step.status} iconSize={20} variant="timeline" />
              <Text as="div" variant="body">
                {step.name}
              </Text>
            </div>
            <Status status={step.status} variant="badge">
              {statusTitle(step.status || "unknown")}
            </Status>
          </div>

          {step.group.can_approve && step.group.approval ? (
            <Button
              variant="primary"
              size="sm"
              disabled={actionPending || isReadOnly}
              onClick={() => void onApprove(step.group)}
            >
              Approve plan
            </Button>
          ) : null}

          {step.group.can_retry && step.group.retry_step_id ? (
            <Button
              variant="primary"
              size="sm"
              onClick={() => {
                onRetry(step.group);
              }}
            >
              Retry
            </Button>
          ) : null}
        </li>
      ))}
    </ol>
  );
}
