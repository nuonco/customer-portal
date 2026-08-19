import { beforeEach, expect, test } from "bun:test";
import { GlobalRegistrator } from "@happy-dom/global-registrator";
import { render, screen } from "@testing-library/react";

import type {
  TCustomerWizardGroup,
  TCustomerWizardState,
} from "@/lib/api/customer/install-wizard";
import { WorkflowStepsList } from "./WorkflowStepsList";

// Other files in this suite replace globalThis.window with a stub and bun shares
// one process, so restore a real DOM before rendering. See Input.test.tsx.
beforeEach(() => {
  const w = globalThis.window as unknown as { document?: unknown } | undefined;
  if (!w?.document) {
    GlobalRegistrator.register();
  }
  document.body.innerHTML = "";
});

type Step = TCustomerWizardGroup["steps"][number];

function step(overrides: Partial<Step>): Step {
  return {
    id: "stp_1",
    name: "step",
    execution_type: "system",
    status: "pending",
    retryable: false,
    finished: false,
    ...overrides,
  } as Step;
}

function workflow(steps: Step[]): TCustomerWizardState["workflow"] {
  return {
    groups: [
      {
        id: "grp_1",
        title: "provision-sandbox",
        domain: "sandbox",
        status: "approval-awaiting",
        can_approve: false,
        steps,
      },
    ],
  } as unknown as TCustomerWizardState["workflow"];
}

function renderList(steps: Step[]) {
  render(
    <WorkflowStepsList
      workflow={workflow(steps)}
      actionPending={false}
      onApprove={async () => {}}
      onRetry={async () => {}}
    />,
  );
}

// The reported bug: "provision sandbox apply plan" was labelled Running while its
// plan was still awaiting approval, because `pending` — which means the step has
// not started — was treated as an active status.
test("a pending step reads Pending, not Running", () => {
  renderList([
    step({ id: "apply", name: "provision sandbox apply plan", status: "pending" }),
  ]);

  expect(screen.queryByText("Running")).toBeNull();
  // Status renders the label twice: hidden for the timeline icon, plus the badge.
  expect(screen.queryAllByText("Pending").length).toBeGreaterThan(0);
});

test("an awaiting-approval step keeps its own label", () => {
  renderList([
    step({
      id: "plan",
      name: "provision sandbox plan",
      execution_type: "approval",
      status: "approval-awaiting",
    }),
  ]);

  expect(screen.queryByText("Running")).toBeNull();
  expect(screen.queryAllByText("Approval awaiting").length).toBeGreaterThan(0);
});

// The exact combination from the bug report.
test("an awaiting plan beside a pending apply shows neither as Running", () => {
  renderList([
    step({
      id: "plan",
      name: "provision sandbox plan",
      execution_type: "approval",
      status: "approval-awaiting",
    }),
    step({ id: "apply", name: "provision sandbox apply plan", status: "pending" }),
  ]);

  expect(screen.queryByText("Running")).toBeNull();
  // Status renders the label twice: hidden for the timeline icon, plus the badge.
  expect(screen.queryAllByText("Pending").length).toBeGreaterThan(0);
});
