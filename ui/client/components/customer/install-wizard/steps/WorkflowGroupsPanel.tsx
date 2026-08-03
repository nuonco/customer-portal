import { Badge } from "@/components/common/Badge";
import { Button } from "@/components/common/Button";
import { Card } from "@/components/common/Card";
import { Loading } from "@/components/common/Loading";
import { Message } from "@/components/common/Message";
import { Status } from "@/components/common/Status";
import { Text } from "@/components/common/Text";
import type {
  TCustomerWizardGroup,
  TCustomerWizardState,
} from "@/lib/api/customer/install-wizard";
import { planSummaryLabel, statusTitle, statusTheme } from "../wizard-utils";
import {
  groupDerivedStatus,
  groupsProgressSummary,
  isActiveStatus,
} from "../wizard-utils";

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

export function WorkflowGroupsPanel({
  workflow,
  actionPending,
  isReadOnly = false,
  onApprove,
  onRetry,
  showActivityIndicators = false,
}: {
  workflow: TCustomerWizardState["workflow"];
  showActivityIndicators?: boolean;
  isReadOnly?: boolean;
} & TWorkflowActionHandlers) {
  return workflow.groups.map((group) => (
    <Card key={group.id} className="bg-surface">
      {(() => {
        const derivedStatus = groupDerivedStatus(group);

        return (
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              {Boolean(group.title) && (
                <Text as="h3" variant="h3" weight="stronger">
                  {group.title}
                </Text>
              )}

              <div className="mt-2 flex flex-wrap items-center gap-2">
                <Status status={derivedStatus}>
                  {statusTitle(derivedStatus)}
                </Status>
                {group.plan_summary ? (
                  <Badge theme="info">
                    {planSummaryLabel(group.plan_summary)}
                  </Badge>
                ) : null}
                <Badge theme="neutral">{groupsProgressSummary([group])}</Badge>

                {group.image_tag ? (
                  <Badge theme="neutral">{group.image_tag}</Badge>
                ) : null}
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              {group.can_approve && group.approval ? (
                <Button
                  variant="primary"
                  size="sm"
                  disabled={actionPending || isReadOnly}
                  onClick={() => void onApprove(group)}
                >
                  Approve plan
                </Button>
              ) : null}
              {group.can_retry && group.retry_step_id ? (
                <Button
                  variant="primary"
                  size="sm"
                  disabled={actionPending || isReadOnly}
                  onClick={() => void onRetry(group)}
                >
                  Retry step
                </Button>
              ) : null}
            </div>
          </div>
        );
      })()}

      {group.active_step_name || group.active_step_description ? (
        <div className="rounded-xl bg-surface-muted px-4 py-3">
          <Text as="div" variant="body" weight="stronger">
            {group.active_step_name || "Processing"}
          </Text>
          {group.active_step_description ? (
            <Text as="div" variant="subtext" theme="neutral">
              {group.active_step_description}
            </Text>
          ) : null}
        </div>
      ) : null}

      {group.target_status || group.target_description ? (
        <Message theme={group.target_status === "error" ? "warning" : "info"}>
          {group.target_description || statusTitle(group.target_status || "")}
        </Message>
      ) : null}

      <div className="space-y-2">
        {group.steps.map((step) => {
          const isStepWorking = isActiveStatus(step.status || "");

          return (
            <div
              key={step.id}
              className="flex items-start justify-between gap-3 rounded-xl border border-border-subtle px-3 py-3"
            >
              <div>
                <Text as="div" variant="body" weight="stronger">
                  {step.name}
                </Text>
                <Text as="div" variant="subtext" theme="neutral">
                  {step.execution_type}
                </Text>
                {step.status_human_description ? (
                  <Text as="div" variant="subtext" theme="neutral">
                    {step.status_human_description}
                  </Text>
                ) : null}
              </div>
              {showActivityIndicators && isStepWorking ? (
                <Badge theme="info">
                  <Loading className="h-3.5 w-3.5" />
                  Running
                </Badge>
              ) : (
                <Badge theme={statusTheme(step.status)}>
                  {statusTitle(step.status || "unknown")}
                </Badge>
              )}
            </div>
          );
        })}
      </div>
    </Card>
  ));
}
