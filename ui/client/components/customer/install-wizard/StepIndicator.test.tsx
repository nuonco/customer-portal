import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { render, screen, waitFor } from "@testing-library/react";
import { CustomerInstallWizardStepIndicator } from "./StepIndicator";
import type { TCustomerWizardStep } from "@/lib/api/customer/install-wizard";
import { InstallWizardContext } from "@/providers/install-wizard-provider";

type TStepIndicatorState = {
  status: string;
  accessible: boolean;
};

const baseSteps: Record<TCustomerWizardStep, TStepIndicatorState> = {
  inputs: { status: "completed", accessible: true },
  stack: { status: "completed", accessible: true },
  sandbox: { status: "in-progress", accessible: true },
  components: { status: "upcoming", accessible: true },
};

const originalGetBoundingClientRect = HTMLElement.prototype.getBoundingClientRect;
const originalResizeObserver = globalThis.ResizeObserver;

beforeAll(() => {
  class ResizeObserverMock {
    observe() {}
    unobserve() {}
    disconnect() {}
  }

  globalThis.ResizeObserver = ResizeObserverMock as typeof ResizeObserver;

  HTMLElement.prototype.getBoundingClientRect = function getBoundingClientRectMock() {
    const isContainer = this.getAttribute("data-step-container") === "true";
    if (isContainer) {
      return {
        x: 100,
        y: 200,
        left: 100,
        top: 200,
        right: 700,
        bottom: 320,
        width: 600,
        height: 120,
        toJSON: () => ({}),
      } as DOMRect;
    }

    const stepIndexAttr = this.getAttribute("data-step-index");
    if (stepIndexAttr !== null) {
      const index = Number(stepIndexAttr);
      const left = 120 + index * 140;
      const top = 212;
      const width = 130;
      const height = 56;

      return {
        x: left,
        y: top,
        left,
        top,
        right: left + width,
        bottom: top + height,
        width,
        height,
        toJSON: () => ({}),
      } as DOMRect;
    }

    return originalGetBoundingClientRect.call(this);
  };
});

afterAll(() => {
  HTMLElement.prototype.getBoundingClientRect = originalGetBoundingClientRect;
  globalThis.ResizeObserver = originalResizeObserver;
});

describe("CustomerInstallWizardStepIndicator", () => {
  test("moves active background when current step changes", async () => {
    const { rerender } = render(
      <CustomerInstallWizardStepIndicator currentStep="inputs" steps={baseSteps} />,
    );

    const activeBackground = screen.getByTestId("step-indicator-active-bg");

    await waitFor(() => {
      expect(activeBackground).toHaveStyle({
        transform: "translate3d(20px, 12px, 0)",
        width: "130px",
        height: "56px",
      });
    });
    expect(activeBackground).toHaveClass("transition-none");

    rerender(
      <CustomerInstallWizardStepIndicator currentStep="sandbox" steps={baseSteps} />,
    );

    await waitFor(() => {
      expect(activeBackground).toHaveStyle({
        transform: "translate3d(300px, 12px, 0)",
        width: "130px",
        height: "56px",
      });
    });
    expect(activeBackground).toHaveClass("transition-transform");
    expect(activeBackground).not.toHaveClass("transition-[transform,width,height,opacity]");
  });

  test("marks deploy components as completed when view install is enabled", () => {
    render(
      <InstallWizardContext.Provider
        value={{
          appId: "app_1",
          currentStep: "components",
          installId: "inst_1",
          workflowId: "wf_1",
          stepStates: {
            inputs: { status: "completed", accessible: true },
            stack: { status: "completed", accessible: true },
            sandbox: { status: "completed", accessible: true },
            components: { status: "in-progress", accessible: true },
          },
          shouldPoll: false,
          wizardState: {
            app: {
              app_id: "app_1",
              display_name: "Demo app",
              summary: "",
              status: "ready",
              logo_light: "",
              logo_dark: "",
              platform: "aws",
            },
            workflow: {
              install: {
                id: "inst_1",
                name: "Install",
                status: "active",
                region: "us-east-1",
                created_at: "2026-01-01T00:00:00Z",
              },
              groups: [],
              policy_totals: { pass: 0, warn: 0, deny: 0 },
              has_approval_awaiting: false,
              is_step_complete: true,
              is_step_error: false,
              overview_path: "/installs/inst_1",
            },
          },
          isLoading: false,
          loadError: null,
          createError: null,
          actionError: null,
          isCreating: false,
          actionPending: false,
          confirmInstall: () => {},
          moveToNextStep: () => {},
          goToStep: () => {},
          approve: async () => {},
          approveAll: async () => {},
          retry: async () => {},
        }}
      >
        <CustomerInstallWizardStepIndicator />
      </InstallWizardContext.Provider>,
    );

    const componentsButton = screen.getByText("Deploy components").closest("button");
    const statusCircle = componentsButton?.querySelector(".rounded-full");
    expect(statusCircle).toHaveClass("bg-green-50");
  });

  test("keeps deploy components upcoming when user has not reached it", () => {
    render(
      <InstallWizardContext.Provider
        value={{
          appId: "app_1",
          currentStep: "sandbox",
          installId: "inst_1",
          workflowId: "wf_1",
          stepStates: {
            inputs: { status: "completed", accessible: true },
            stack: { status: "completed", accessible: true },
            sandbox: { status: "in-progress", accessible: true },
            components: { status: "upcoming", accessible: false },
          },
          shouldPoll: false,
          wizardState: {
            app: {
              app_id: "app_1",
              display_name: "Demo app",
              summary: "",
              status: "ready",
              logo_light: "",
              logo_dark: "",
              platform: "aws",
            },
            workflow: {
              install: {
                id: "inst_1",
                name: "Install",
                status: "active",
                region: "us-east-1",
                created_at: "2026-01-01T00:00:00Z",
              },
              groups: [],
              policy_totals: { pass: 0, warn: 0, deny: 0 },
              has_approval_awaiting: false,
              is_step_complete: true,
              is_step_error: false,
              overview_path: "/installs/inst_1",
            },
          },
          isLoading: false,
          loadError: null,
          createError: null,
          actionError: null,
          isCreating: false,
          actionPending: false,
          confirmInstall: () => {},
          moveToNextStep: () => {},
          goToStep: () => {},
          approve: async () => {},
          approveAll: async () => {},
          retry: async () => {},
        }}
      >
        <CustomerInstallWizardStepIndicator />
      </InstallWizardContext.Provider>,
    );

    const componentsButton = screen.getByText("Deploy components").closest("button");
    const statusCircle = componentsButton?.querySelector(".rounded-full");
    expect(statusCircle).toHaveClass("bg-neutral-50");
  });
});
