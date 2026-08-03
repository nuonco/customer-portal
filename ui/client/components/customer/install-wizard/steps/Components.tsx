import { Badge } from "@/components/common/Badge";
import { Button } from "@/components/common/Button";
import { Card } from "@/components/common/Card";
import { Icon } from "@/components/common/Icon";
import { Status } from "@/components/common/Status";
import { Text } from "@/components/common/Text";
import type { TCustomerWizardState } from "@/lib/api/customer/install-wizard";
import { planSummaryLabel } from "../wizard-utils";
import {
  type TWorkflowActionHandlers,
  WaitingForWorkflowDetailsCard,
  WorkflowGroupsPanel,
} from "./WorkflowGroupsPanel";

export function WorkflowComponentsStep({
  title,
  headerStatus,
  headerStatusText,
  progressSummary,
  showApproveAll,
  showNextStepButton,
  showViewInstallButton,
  hasStepError,
  nextStepLabel,
  workflow,
  showWaitingForWorkflowDetails,
  actionPending,
  isReadOnly,
  onNext,
  onApproveAll,
  onApprove,
  onRetry,
}: {
  title: string;
  headerStatus: string;
  headerStatusText: string;
  progressSummary: string;
  showApproveAll: boolean;
  showNextStepButton: boolean;
  showViewInstallButton: boolean;
  hasStepError: boolean;
  nextStepLabel: string;
  workflow: TCustomerWizardState["workflow"];
  showWaitingForWorkflowDetails: boolean;
  isReadOnly: boolean;
  onNext: () => void;
  onApproveAll: () => Promise<void>;
} & TWorkflowActionHandlers) {
  return (
    <div className="space-y-4">
      <Card className="bg-surface">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex items-center gap-2">
            <Text as="h3" variant="h3" weight="stronger">
              {title}
            </Text>
            <div className="flex flex-wrap items-center gap-4">
              <Status status={headerStatus}>{headerStatusText}</Status>
              {workflow.plan_summary ? (
                <Badge theme="info">
                  {planSummaryLabel(workflow.plan_summary)}
                </Badge>
              ) : null}
              <Badge theme="neutral">{progressSummary}</Badge>
              {workflow.policy_totals.warn > 0 ? (
                <Badge theme="warn">{workflow.policy_totals.warn} warn</Badge>
              ) : null}
              {workflow.policy_totals.deny > 0 ? (
                <Badge theme="error">{workflow.policy_totals.deny} deny</Badge>
              ) : null}
              {workflow.policy_totals.pass > 0 ? (
                <Badge theme="success">{workflow.policy_totals.pass} pass</Badge>
              ) : null}
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            {showApproveAll ? (
              <Button
                variant="secondary"
                disabled={actionPending || isReadOnly || !workflow.has_approval_awaiting}
                onClick={() => void onApproveAll()}
              >
                Approve all
              </Button>
            ) : null}
            {showNextStepButton ? (
              <Button
                variant="primary"
                size="md"
                disabled={isReadOnly || !workflow.is_step_complete}
                onClick={onNext}
              >
                {nextStepLabel}
                <Icon variant="ArrowRightIcon" size={16} weight="bold" />
              </Button>
            ) : null}
            {showViewInstallButton ? (
              <Button
                href={workflow.overview_path}
                variant="primary"
                size="md"
                className={
                  !workflow.is_step_complete && !hasStepError
                    ? "opacity-40 pointer-events-none"
                    : undefined
                }
                aria-disabled={!workflow.is_step_complete && !hasStepError}
              >
                <Icon variant="CheckCircleIcon" size={16} weight="bold" />
                View install
              </Button>
            ) : null}
          </div>
        </div>
      </Card>

      {showWaitingForWorkflowDetails ? (
        <WaitingForWorkflowDetailsCard />
      ) : (
        <WorkflowGroupsPanel
          workflow={workflow}
          actionPending={actionPending}
          isReadOnly={isReadOnly}
          onApprove={onApprove}
          onRetry={onRetry}
          showActivityIndicators
        />
      )}
    </div>
  );
}
