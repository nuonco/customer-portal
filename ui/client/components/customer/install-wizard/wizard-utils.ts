import type {
  TCustomerWizardGroup,
  TCustomerWizardForm,
  TCustomerWizardInputField,
  TCustomerWizardPlanSummary,
  TCustomerWizardState,
  TCustomerWizardStep,
} from "@/lib/api/customer/install-wizard";

export function isBooleanField(field: TCustomerWizardInputField) {
  return field.type === "bool" || field.default === "true" || field.default === "false";
}

export function statusTheme(status: string): "success" | "warn" | "error" | "neutral" {
  if (status === "completed" || status === "success") {
    return "success";
  }
  if (status === "approval-awaiting") {
    return "warn";
  }
  if (status === "error") {
    return "error";
  }
  return "neutral";
}

export function statusTitle(status: string) {
  return status
    .split("-")
    .join(" ")
    .split("_")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export function planSummaryLabel(plan: TCustomerWizardPlanSummary) {
  if (plan.kind === "helm") {
    return `${plan.add_count} add, ${plan.change_count} change, ${plan.destroy_count} destroy`;
  }
  return `${plan.create_count} create, ${plan.update_count} update, ${plan.delete_count} delete, ${plan.replace_count} replace`;
}

export type TInstallWizardFormValues = {
  installName: string;
  region: string;
  location: string;
  inputs: Record<string, string>;
};

export function buildInitialWizardValues(form: TCustomerWizardForm): TInstallWizardFormValues {
  const initialInputs: Record<string, string> = {};
  const groups = Array.isArray(form.input_groups) ? form.input_groups : [];

  groups.forEach((group) => {
    const inputs = Array.isArray(group?.inputs) ? group.inputs : [];
    inputs.forEach((input) => {
      initialInputs[input.name] = input.default ?? (isBooleanField(input) ? "false" : "");
    });
  });

  return {
    installName: form.install_name ?? "",
    region: "us-east-1",
    location: "eastus",
    inputs: initialInputs,
  };
}

export type TWorkflowViewModel = {
  currentStep: TCustomerWizardStep;
  currentStepTitle: string;
  aggregateStatus: string;
  headerStatus: string;
  headerStatusText: string;
  nextStepLabel: string;
  isStackStep: boolean;
  isSandboxStep: boolean;
  isComponentsStep: boolean;
  stackHasContent: boolean;
  isStackComplete: boolean;
  showStackSkeleton: boolean;
  showWaitingForWorkflowDetails: boolean;
  showApproveAll: boolean;
  showNextStepButton: boolean;
  hasStepError: boolean;
  showViewInstallButton: boolean;
  progressSummary: string;
};

export function normalizeStatus(status: string) {
  return (status || "").trim().toLowerCase();
}

export function isTerminalSuccessStatus(status: string) {
  switch (normalizeStatus(status)) {
    case "completed":
    case "success":
    case "approved":
    case "cancelled":
    case "skipped":
      return true;
    default:
      return false;
  }
}

/**
 * Whether a step is executing *right now*.
 *
 * The API's step vocabulary is "finished" | "pending" | "in-progress" |
 * "timed-out" | "error", where `pending` means the step has not started yet —
 * typically because it is blocked behind an approval. This used to include
 * `pending`/`queued`/`not_started`, which made not-yet-started steps render as a
 * "Running" spinner (e.g. "provision sandbox apply plan" showing Running while
 * its plan was still awaiting approval).
 *
 * Use isUnfinishedStatus when the question is "is there still work to do".
 */
export function isActiveStatus(status: string) {
  switch (normalizeStatus(status)) {
    case "in-progress":
    case "active":
    case "running":
      return true;
    default:
      return false;
  }
}

/** Whether a step is waiting its turn and has not started executing. */
export function isPendingStatus(status: string) {
  switch (normalizeStatus(status)) {
    case "pending":
    case "queued":
    case "not_started":
      return true;
    default:
      return false;
  }
}

/**
 * Whether a step still has work outstanding — running or merely queued. This is
 * what aggregate/progress logic wants; isActiveStatus is only for "is it moving".
 */
export function isUnfinishedStatus(status: string) {
  return isActiveStatus(status) || isPendingStatus(status);
}

export function statusText(status: string) {
  switch (status) {
    case "completed":
    case "success":
    case "approved":
    case "cancelled":
    case "skipped":
      return "Completed";
    case "in-progress":
    case "active":
    case "running":
      return "In progress";
    case "pending":
    case "queued":
    case "not_started":
      return "Waiting";
    case "approval-awaiting":
      return "Awaiting approval";
    case "error":
      return "Failed";
    default:
      return "Waiting";
  }
}

export function latestSteps(group: TCustomerWizardGroup) {
  const seen = new Set<string>();
  const latest: TCustomerWizardGroup["steps"] = [];

  for (let idx = group.steps.length - 1; idx >= 0; idx -= 1) {
    const step = group.steps[idx];
    const key = step.execution_type || step.id;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    latest.push(step);
  }

  latest.reverse();
  return latest;
}

export function groupDerivedStatus(group: TCustomerWizardGroup) {
  if (normalizeStatus(group.target_status || "") === "error") {
    return "error";
  }

  const latest = latestSteps(group);
  if (latest.length === 0) {
    return group.status || "in-progress";
  }

  let hasActive = false;
  let allTerminalSuccess = true;
  for (const step of latest) {
    const status = normalizeStatus(step.status || "not_started");
    if (status === "error") {
      return "error";
    }
    if (status === "approval-awaiting") {
      return "approval-awaiting";
    }
    // Deliberately the union: a group with only queued steps is still in
    // progress overall, and polling keys off this value.
    if (isUnfinishedStatus(status)) {
      hasActive = true;
    }
    if (!isTerminalSuccessStatus(status)) {
      allTerminalSuccess = false;
    }
  }

  if (hasActive) {
    return "in-progress";
  }
  if (allTerminalSuccess) {
    return "completed";
  }
  return group.status || "in-progress";
}

export function groupsProgressSummary(groups: TCustomerWizardGroup[]) {
  let done = 0;
  let total = 0;

  for (const group of groups) {
    const latest = latestSteps(group);
    done += latest.filter((step) => isTerminalSuccessStatus(step.status || "")).length;
    total += latest.length;
  }

  if (total === 0) {
    return "0/0 steps complete";
  }

  return `${done}/${total} steps complete`;
}

export function aggregateGroupStatus(groups: TCustomerWizardGroup[]) {
  if (groups.length === 0) {
    return "in-progress";
  }

  let hasActive = false;
  for (const group of groups) {
    const status = groupDerivedStatus(group);
    if (status === "error") {
      return "error";
    }
    if (status === "approval-awaiting") {
      return "approval-awaiting";
    }
    if (!isTerminalSuccessStatus(status)) {
      hasActive = true;
    }
  }

  return hasActive ? "in-progress" : "completed";
}

export function normalizeStepStateStatus(status: string | undefined) {
  const normalized = normalizeStatus(status || "");
  return normalized || "upcoming";
}

export function stepTitle(step: string) {
  switch (step) {
    case "stack":
      return "Deploy stack";
    case "sandbox":
      return "Deploy sandbox";
    case "components":
      return "Deploy components";
    default:
      return "Workflow";
  }
}

export function nextLabel(step: string) {
  switch (step) {
    case "stack":
      return "Deploy sandbox";
    case "sandbox":
      return "Deploy components";
    default:
      return "Next";
  }
}

export function stackInProgressLabel(groups: TCustomerWizardGroup[]) {
  for (const group of groups) {
    for (const step of group.steps) {
      if (
        step.status === "completed" ||
        step.status === "success" ||
        step.status === "approved" ||
        step.status === "error"
      ) {
        continue;
      }

      if (step.name === "generate install stack") {
        return "Generating stack";
      }
      if (step.name === "await install stack") {
        return "Awaiting stack";
      }
      if (step.name === "update install stack outputs") {
        return "Updating stack outputs";
      }
    }
  }

  return "In progress";
}

export function sandboxActiveLabel(groups: TCustomerWizardGroup[]) {
  if (groups.length === 0) {
    return "";
  }

  const group = groups[0];
  const planStep = [...group.steps]
    .reverse()
    .find((step) => step.execution_type === "approval");
  const applyStep = [...group.steps]
    .reverse()
    .find((step) => step.execution_type !== "approval");

  const planStatus = planStep?.status ?? "";
  if (isTerminalSuccessStatus(planStatus)) {
    return statusText(applyStep?.status ?? "");
  }

  return statusText(planStatus);
}

export function componentsActiveLabel(groups: TCustomerWizardGroup[]) {
  for (const group of groups) {
    if (groupDerivedStatus(group) === "error") {
      continue;
    }

    const componentName = group.title || "component";
    const planStep = [...group.steps]
      .reverse()
      .find((step) => step.execution_type === "approval");

    if (isActiveStatus(planStep?.status || "")) {
      return `Planning ${componentName} deploy`;
    }
    if (planStep?.status === "approval-awaiting") {
      return `Awaiting approval for ${componentName}`;
    }
  }

  for (const group of groups) {
    if (groupDerivedStatus(group) === "error") {
      continue;
    }

    const componentName = group.title || "component";
    const applyStep = [...group.steps]
      .reverse()
      .find((step) => step.execution_type !== "approval");
    if (isActiveStatus(applyStep?.status || "")) {
      return `Deploying ${componentName}`;
    }
  }

  return "";
}

export function headerStatusLabel(
  step: string,
  groups: TCustomerWizardGroup[],
  aggregateStatus: string,
) {
  if (step === "stack" && aggregateStatus === "in-progress") {
    return stackInProgressLabel(groups);
  }

  if (step === "sandbox" && aggregateStatus !== "completed") {
    return sandboxActiveLabel(groups) || statusText(aggregateStatus);
  }

  if (step === "components" && aggregateStatus !== "completed") {
    return componentsActiveLabel(groups) || statusText(aggregateStatus);
  }

  return statusText(aggregateStatus);
}

export function hasStackSetupContent(
  stackSetup: TCustomerWizardState["workflow"]["stack_setup"] | undefined,
) {
  if (!stackSetup) {
    return false;
  }

  return Boolean(
    stackSetup.cloudformation_link ||
      stackSetup.template_url ||
      stackSetup.tfvars_content ||
      stackSetup.azure_template_url,
  );
}

export function isS3Template(templateUrl: string) {
  return templateUrl.includes("s3.amazonaws.com") || templateUrl.includes(".s3.");
}

export function createStackCmd(templateUrl: string, stackName: string, region: string) {
  if (isS3Template(templateUrl)) {
    return `aws cloudformation create-stack \
  --stack-name ${stackName} \
  --template-url ${templateUrl} \
  --capabilities CAPABILITY_NAMED_IAM \
  --region ${region}`;
  }

  return `curl -sLo template.json "${templateUrl}" \
  && aws cloudformation create-stack \
  --stack-name ${stackName} \
  --template-body file://template.json \
  --capabilities CAPABILITY_NAMED_IAM \
  --region ${region}`;
}

export function updateStackCmd(templateUrl: string, stackName: string, region: string) {
  if (isS3Template(templateUrl)) {
    return `aws cloudformation update-stack \
  --stack-name ${stackName} \
  --template-url ${templateUrl} \
  --capabilities CAPABILITY_NAMED_IAM \
  --region ${region}`;
  }

  return `curl -sLo template.json "${templateUrl}" \
  && aws cloudformation update-stack \
  --stack-name ${stackName} \
  --template-body file://template.json \
  --capabilities CAPABILITY_NAMED_IAM \
  --region ${region}`;
}

export function consoleUrl(stackName: string, region: string) {
  return `https://console.aws.amazon.com/cloudformation/home?region=${region}#/stacks/events?filteringText=${stackName}&filteringStatus=active&viewNested=true`;
}

export function gcpBackendSnippet(installID: string) {
  return [
    "terraform {",
    '  backend "gcs" {',
    '    bucket = "<your-state-bucket>"',
    `    prefix = "nuon/${installID}"`,
    "  }",
    "}",
  ].join("\n");
}

export function gcpApplyCmd() {
  return "terraform init && terraform apply -var-file=install.tfvars";
}

export function buildWorkflowViewModel(
  wizardState: TCustomerWizardState,
  currentStep: TCustomerWizardStep,
  shouldPoll: boolean,
): TWorkflowViewModel {
  const workflow = wizardState.workflow;
  if (!workflow) {
    throw new Error("buildWorkflowViewModel requires wizardState.workflow");
  }

  const isStackStep = currentStep === "stack";
  const isSandboxStep = currentStep === "sandbox";
  const isComponentsStep = currentStep === "components";

  const aggregateStatus = (() => {
    const statusFromGroups = aggregateGroupStatus(workflow.groups);
    if (workflow.groups.length > 0) {
      return statusFromGroups;
    }

    if (workflow.is_step_error) {
      return "error";
    }

    if (workflow.is_step_complete) {
      return "completed";
    }

    if (shouldPoll) {
      return "in-progress";
    }

    return statusFromGroups;
  })();

  const stackHasContent = hasStackSetupContent(workflow.stack_setup);
  const isStackComplete =
    isStackStep && (workflow.is_step_complete || aggregateStatus === "completed");
  const showStackSkeleton = isStackStep && !stackHasContent;
  const showWaitingForWorkflowDetails =
    !isStackStep &&
    shouldPoll &&
    workflow.groups.length === 0 &&
    !isStackComplete;
  const showApproveAll = (isSandboxStep || isComponentsStep) && workflow.groups.length > 0;
  const showNextStepButton = (isStackStep || isSandboxStep) && Boolean(workflow.next_step);
  const hasStepError = workflow.is_step_error || aggregateStatus === "error";
  const showViewInstallButton = isComponentsStep && Boolean(workflow.overview_path);
  const progressSummary = groupsProgressSummary(workflow.groups);

  const headerStatus = (() => {
    if (hasStepError) {
      return "error";
    }

    if (isComponentsStep) {
      const activeLabel = componentsActiveLabel(workflow.groups);
      if (activeLabel.startsWith("Awaiting approval")) {
        return "approval-awaiting";
      }
      if (activeLabel) {
        return "in-progress";
      }
    }

    return aggregateStatus;
  })();

  return {
    currentStep,
    currentStepTitle: stepTitle(currentStep),
    aggregateStatus,
    headerStatus,
    headerStatusText: headerStatusLabel(currentStep, workflow.groups, aggregateStatus),
    nextStepLabel: nextLabel(currentStep),
    isStackStep,
    isSandboxStep,
    isComponentsStep,
    stackHasContent,
    isStackComplete,
    showStackSkeleton,
    showWaitingForWorkflowDetails,
    showApproveAll,
    showNextStepButton,
    hasStepError,
    showViewInstallButton,
    progressSummary,
  };
}
