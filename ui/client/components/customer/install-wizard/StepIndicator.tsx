import { useCallback, useContext, useEffect, useRef, useState } from "react";
import { Icon } from "@/components/common/Icon";
import { Text } from "@/components/common/Text";
import { cn } from "@/utils/classnames";
import type { TCustomerWizardStep } from "@/lib/api/customer/install-wizard";
import { InstallWizardContext } from "@/providers/install-wizard-provider";
import { aggregateGroupStatus } from "./wizard-utils";

type TStepIndicatorState = {
  status: string;
  accessible: boolean;
};

const STEP_ORDER: Array<{ key: TCustomerWizardStep; label: string }> = [
  { key: "inputs", label: "Configure install" },
  { key: "stack", label: "Deploy stack" },
  { key: "sandbox", label: "Deploy sandbox" },
  { key: "components", label: "Deploy components" },
];

function StepIndicator({ status, index }: { status: string; index: number }) {
  if (status === "completed" || status === "success") {
    return <Icon variant="CheckIcon" size={16} />;
  }
  if (status === "in-progress") {
    return <Icon variant="CircleNotchIcon" size={16} />;
  }
  if (status === "approval-awaiting") {
    return <Icon variant="BellRingingIcon" size={16} />;
  }
  if (status === "error") {
    return <Icon variant="WarningIcon" size={16} />;
  }

  return <div className="text-[13px]">{index + 1}</div>;
}

function getStepStatusClasses(status?: string) {
  if (status === "completed" || status === "success") {
    return "!border-green-300 bg-green-50 text-green-500";
  }
  if (status === "in-progress" || status === "approval-awaiting") {
    return "!border-blue-200 bg-blue-50 text-blue-500";
  }
  if (status === "error") {
    return "!border-red-300 bg-red-50 text-red-500";
  }

  return "!border-neutral-200 bg-neutral-50 text-text-muted opacity-80";
}

export function CustomerInstallWizardStepIndicator({
  currentStep: providedCurrentStep,
  steps: providedSteps,
  onStepSelect: providedOnStepSelect,
}: {
  currentStep?: TCustomerWizardStep;
  steps?: Record<TCustomerWizardStep, TStepIndicatorState>;
  onStepSelect?: (step: TCustomerWizardStep) => void;
} = {}) {
  const context = useContext(InstallWizardContext);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const stepRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const previousStepIndexRef = useRef<number | null>(null);
  const [shouldAnimateBg, setShouldAnimateBg] = useState(false);
  const [activeRect, setActiveRect] = useState<{
    left: number;
    top: number;
    width: number;
    height: number;
  } | null>(null);
  if (!providedCurrentStep && !providedSteps && !context) {
    throw new Error(
      "CustomerInstallWizardStepIndicator must be used within InstallWizardProvider",
    );
  }
  const currentStep = providedCurrentStep ?? context?.currentStep;
  const stepStates = providedSteps ?? context?.stepStates;
  const onStepSelect = providedOnStepSelect ?? context?.goToStep;

  if (!currentStep || !stepStates) {
    throw new Error("CustomerInstallWizardStepIndicator requires step state");
  }

  const workflow = context?.wizardState?.workflow;
  const hasReachedComponentsStep =
    stepStates.components?.status !== "upcoming" ||
    Boolean(stepStates.components?.accessible);
  const isViewInstallEnabled = (() => {
    if (!workflow?.overview_path) {
      return false;
    }

    const hasStepError =
      workflow.is_step_error ||
      (workflow.groups.length > 0 && aggregateGroupStatus(workflow.groups) === "error");

    return workflow.is_step_complete || hasStepError;
  })();

  const displayStepStates =
    isViewInstallEnabled && hasReachedComponentsStep && stepStates.components
      ? {
          ...stepStates,
          components: {
            ...stepStates.components,
            status: "completed",
            accessible: true,
          },
        }
      : stepStates;

  const currentStepIndex = STEP_ORDER.findIndex(
    (step) => step.key === currentStep,
  );

  const updateActiveRect = useCallback(() => {
    const container = containerRef.current;
    const activeStep = stepRefs.current[currentStepIndex];

    if (!container || !activeStep) {
      setActiveRect(null);
      return;
    }

    const containerRect = container.getBoundingClientRect();
    const activeStepRect = activeStep.getBoundingClientRect();

    setActiveRect({
      left: activeStepRect.left - containerRect.left,
      top: activeStepRect.top - containerRect.top,
      width: activeStepRect.width,
      height: activeStepRect.height,
    });
  }, [currentStepIndex]);

  useEffect(() => {
    if (previousStepIndexRef.current === null) {
      previousStepIndexRef.current = currentStepIndex;
      return;
    }

    if (previousStepIndexRef.current !== currentStepIndex) {
      setShouldAnimateBg(true);
      previousStepIndexRef.current = currentStepIndex;
    }
  }, [currentStepIndex]);

  useEffect(() => {
    updateActiveRect();

    const container = containerRef.current;
    if (!container) {
      return;
    }

    const resizeObserver =
      typeof ResizeObserver !== "undefined"
        ? new ResizeObserver(() => {
            updateActiveRect();
          })
        : null;

    if (resizeObserver) {
      resizeObserver.observe(container);
      stepRefs.current.forEach((stepRef) => {
        if (stepRef) {
          resizeObserver.observe(stepRef);
        }
      });
    }

    window.addEventListener("resize", updateActiveRect);

    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", updateActiveRect);
    };
  }, [updateActiveRect]);

  const isStepClickable = (status?: string) =>
    status === "completed" ||
    status === "success" ||
    status === "in-progress" ||
    status === "approval-awaiting" ||
    status === "error";

  return (
    <div
      ref={containerRef}
      data-step-container="true"
      className="relative grid gap-3 rounded-md border bg-surface p-4 md:grid-cols-4"
    >
      <div
        data-testid="step-indicator-active-bg"
        aria-hidden="true"
        className={cn(
          "pointer-events-none absolute z-0 rounded-lg bg-neutral-100 dark:bg-neutral-800",
          shouldAnimateBg
            ? "transition-transform duration-300 ease-out"
            : "transition-none",
        )}
        style={{
          width: activeRect ? `${activeRect.width}px` : "0px",
          height: activeRect ? `${activeRect.height}px` : "0px",
          transform: activeRect
            ? `translate3d(${activeRect.left}px, ${activeRect.top}px, 0)`
            : "translate3d(0, 0, 0)",
          opacity: activeRect ? 1 : 0,
        }}
      />
      {STEP_ORDER.map((step, index) => {
        const state = displayStepStates[step.key];
        const isClickable = isStepClickable(state?.status);

        return (
          <button
            key={step.key}
            ref={(element) => {
              stepRefs.current[index] = element;
            }}
            data-step-index={index}
            type="button"
            onClick={() => {
              if (isClickable) {
                onStepSelect?.(step.key);
              }
            }}
            disabled={!isClickable}
            className={cn(
              "relative z-10 flex w-full items-center gap-3 rounded-lg bg-transparent p-3 text-left transition-colors",
              state?.accessible ? "text-text-primary" : "text-text-muted",
              isClickable ? "cursor-pointer" : "cursor-not-allowed",
            )}
          >
            <div
              className={cn(
                "shrink-0 flex h-8 w-8 items-center justify-center rounded-full border",
                getStepStatusClasses(state?.status),
              )}
            >
              <StepIndicator
                status={state?.status ?? "upcoming"}
                index={index}
              />
            </div>
            <div>
              <Text as="div" variant="body" weight="strong">
                {step.label}
              </Text>
            </div>
          </button>
        );
      })}
    </div>
  );
}
