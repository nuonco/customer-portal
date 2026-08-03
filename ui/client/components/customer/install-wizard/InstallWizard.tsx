import { useEffect, useRef } from "react";
import { Message } from "@/components/common/Message";
import { Skeleton } from "@/components/common/Skeleton";
import { ConfigureStep, ConfigureStepFromContext } from "./steps/Configure";
import { InstallWizardProvider } from "@/providers/install-wizard-provider";
import { useInstallWizard } from "@/hooks/use-install-wizard";
import { CustomerInstallWizardStepIndicator } from "./StepIndicator";
import { WorkflowComponentsStep } from "./steps/Components";
import { WorkflowSandboxStep } from "./steps/Sandbox";
import { WorkflowStackStep } from "./steps/Stack";
import { buildWorkflowViewModel } from "./wizard-utils";
import type { TCustomerWizardStep } from "@/lib/api/customer/install-wizard";
import "./InstallWizard.css";

export { ConfigureStep };

const STEP_ORDER: TCustomerWizardStep[] = [
  "inputs",
  "stack",
  "sandbox",
  "components",
];

export type TInstallWizardTransitionDirection =
  | "forward"
  | "backward"
  | "none";

function getStepIndex(step: TCustomerWizardStep): number {
  const index = STEP_ORDER.indexOf(step);
  return index === -1 ? 0 : index;
}

export function getStepTransitionDirection(
  previousStep: TCustomerWizardStep,
  currentStep: TCustomerWizardStep,
): TInstallWizardTransitionDirection {
  if (previousStep === currentStep) {
    return "none";
  }

  return getStepIndex(currentStep) > getStepIndex(previousStep)
    ? "forward"
    : "backward";
}

export function CustomerInstallWizard({ appId }: { appId: string }) {
  return (
    <InstallWizardProvider appId={appId}>
      <CustomerInstallWizardContent />
    </InstallWizardProvider>
  );
}

function CustomerInstallWizardContent() {
  const {
    currentStep,
    stepStates,
    shouldPoll,
    wizardState,
    isLoading,
    loadError,
    actionError,
    moveToNextStep,
    approve,
    approveAll,
    retry,
    actionPending,
  } = useInstallWizard();

  const previousStepRef = useRef<TCustomerWizardStep>(currentStep);
  const stepTransitionDirection = getStepTransitionDirection(
    previousStepRef.current,
    currentStep,
  );

  useEffect(() => {
    previousStepRef.current = currentStep;
  }, [currentStep]);

  const stepContent = (() => {
    if (isLoading) {
      return (
        <div className="space-y-4">
          <Skeleton lines={1} height="72px" />
          <Skeleton lines={3} height="32px" />
        </div>
      );
    }

    if (!wizardState) {
      return (
        <Message theme="warning">
          {loadError ?? "Unable to load the install wizard."}
        </Message>
      );
    }

    if (currentStep === "inputs" && wizardState.form) {
      return <ConfigureStepFromContext />;
    }

    if (currentStep !== "inputs" && wizardState.workflow) {
      const workflow = wizardState.workflow;
      const isReadOnly =
        stepStates[currentStep]?.status === "completed" ||
        stepStates[currentStep]?.status === "success";
      const viewModel = buildWorkflowViewModel(
        wizardState,
        currentStep,
        shouldPoll,
      );

      const workflowStepContent = (() => {
        switch (currentStep) {
          case "stack":
            return (
              <WorkflowStackStep
                title={viewModel.currentStepTitle}
                headerStatusText={viewModel.headerStatusText}
                isStackComplete={viewModel.isStackComplete}
                nextStepLabel={viewModel.nextStepLabel}
                showNextStepButton={viewModel.showNextStepButton}
                showViewInstallButton={viewModel.showViewInstallButton}
                hasStepError={viewModel.hasStepError}
                isReadOnly={isReadOnly}
                isStepComplete={workflow.is_step_complete}
                overviewPath={workflow.overview_path}
                onNext={moveToNextStep}
                stackSetup={workflow.stack_setup}
                showStackSkeleton={viewModel.showStackSkeleton}
              />
            );
          case "sandbox":
            return (
              <WorkflowSandboxStep
                title={viewModel.currentStepTitle}
                headerStatus={viewModel.headerStatus}
                headerStatusText={viewModel.headerStatusText}
                progressSummary={viewModel.progressSummary}
                showApproveAll={viewModel.showApproveAll}
                showNextStepButton={viewModel.showNextStepButton}
                showViewInstallButton={viewModel.showViewInstallButton}
                hasStepError={viewModel.hasStepError}
                nextStepLabel={viewModel.nextStepLabel}
                workflow={workflow}
                showWaitingForWorkflowDetails={
                  viewModel.showWaitingForWorkflowDetails
                }
                actionPending={actionPending}
                isReadOnly={isReadOnly}
                onNext={moveToNextStep}
                onApproveAll={approveAll}
                onApprove={approve}
                onRetry={retry}
              />
            );
          case "components":
            return (
              <WorkflowComponentsStep
                title={viewModel.currentStepTitle}
                headerStatus={viewModel.headerStatus}
                headerStatusText={viewModel.headerStatusText}
                progressSummary={viewModel.progressSummary}
                showApproveAll={viewModel.showApproveAll}
                showNextStepButton={viewModel.showNextStepButton}
                showViewInstallButton={viewModel.showViewInstallButton}
                hasStepError={viewModel.hasStepError}
                nextStepLabel={viewModel.nextStepLabel}
                workflow={workflow}
                showWaitingForWorkflowDetails={
                  viewModel.showWaitingForWorkflowDetails
                }
                actionPending={actionPending}
                isReadOnly={isReadOnly}
                onNext={moveToNextStep}
                onApproveAll={approveAll}
                onApprove={approve}
                onRetry={retry}
              />
            );
          default:
            return null;
        }
      })();

      return (
        <div className="flex flex-col gap-4">
          {actionError ? (
            <Message theme="warning">{actionError}</Message>
          ) : null}

          {workflow.has_approval_awaiting ? (
            <Message theme="info">
              This step is waiting for approval before it can continue.
            </Message>
          ) : null}

          {viewModel.hasStepError ? (
            <Message theme="warning">
              One or more workflow groups failed. Review the failing group and
              retry if available.
            </Message>
          ) : null}

          {workflowStepContent}
        </div>
      );
    }

    return null;
  })();

  return (
    <div className="space-y-4">
      <CustomerInstallWizardStepIndicator />
      <div
        key={currentStep}
        className={
          stepTransitionDirection === "none"
            ? undefined
            : `install-wizard-step-transition install-wizard-step-transition--${stepTransitionDirection}`
        }
      >
        {stepContent}
      </div>
    </div>
  );
}
