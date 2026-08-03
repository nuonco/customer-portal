import type {
  TCustomerWizardStep,
} from "@/lib/api/customer/install-wizard";
import { CustomerInstallWizardStepIndicator } from "./StepIndicator";

type TStepIndicatorState = {
  status: string;
  accessible: boolean;
};

export default {
  title: "Customer/Install Wizard/Step Indicator",
};

function createStepStates(
  overrides: Partial<Record<TCustomerWizardStep, TStepIndicatorState>> = {},
): Record<TCustomerWizardStep, TStepIndicatorState> {
  return {
    inputs: { status: "completed", accessible: true },
    stack: { status: "upcoming", accessible: true },
    sandbox: { status: "upcoming", accessible: true },
    components: { status: "upcoming", accessible: true },
    ...overrides,
  };
}

function renderStory(
  currentStep: TCustomerWizardStep,
  steps: Record<TCustomerWizardStep, TStepIndicatorState>,
) {
  return (
    <div className="max-w-6xl">
      <CustomerInstallWizardStepIndicator currentStep={currentStep} steps={steps} />
    </div>
  );
}

export const ConfigureInProgress = () =>
  renderStory(
    "inputs",
    createStepStates({
      inputs: { status: "in-progress", accessible: true },
      stack: { status: "upcoming", accessible: false },
      sandbox: { status: "upcoming", accessible: false },
      components: { status: "upcoming", accessible: false },
    }),
  );

export const SandboxApprovalAwaiting = () =>
  renderStory(
    "sandbox",
    createStepStates({
      inputs: { status: "completed", accessible: true },
      stack: { status: "completed", accessible: true },
      sandbox: { status: "approval-awaiting", accessible: true },
      components: { status: "upcoming", accessible: true },
    }),
  );

export const ComponentsError = () =>
  renderStory(
    "components",
    createStepStates({
      inputs: { status: "completed", accessible: true },
      stack: { status: "success", accessible: true },
      sandbox: { status: "completed", accessible: true },
      components: { status: "error", accessible: true },
    }),
  );
