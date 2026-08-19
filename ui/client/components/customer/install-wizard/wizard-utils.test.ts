import { describe, expect, test } from "bun:test";

import type { TCustomerWizardGroup } from "@/lib/api/customer/install-wizard";
import {
  aggregateGroupStatus,
  groupDerivedStatus,
  isActiveStatus,
  isPendingStatus,
  isUnfinishedStatus,
  statusText,
  statusTitle,
} from "./wizard-utils";

type Step = TCustomerWizardGroup["steps"][number];

function step(overrides: Partial<Step> = {}): Step {
  return {
    id: "stp_1",
    name: "provision sandbox apply plan",
    execution_type: "system",
    status: "pending",
    retryable: false,
    finished: false,
    ...overrides,
  } as Step;
}

function group(overrides: Partial<TCustomerWizardGroup> = {}): TCustomerWizardGroup {
  return {
    id: "grp_1",
    title: "provision-sandbox",
    domain: "sandbox",
    status: "in-progress",
    can_approve: false,
    steps: [],
    ...overrides,
  } as TCustomerWizardGroup;
}

describe("isActiveStatus", () => {
  // The API step vocabulary is finished|pending|in-progress|timed-out|error.
  test("is true only for statuses that are executing right now", () => {
    for (const s of ["in-progress", "active", "running", "IN-PROGRESS"]) {
      expect(isActiveStatus(s)).toBe(true);
    }
  });

  // The reported bug: a not-yet-started step rendered as a "Running" spinner.
  test("is false for statuses that have not started", () => {
    for (const s of ["pending", "queued", "not_started"]) {
      expect(isActiveStatus(s)).toBe(false);
    }
  });

  test("is false for terminal and unknown statuses", () => {
    for (const s of ["finished", "completed", "error", "timed-out", "", "nonsense"]) {
      expect(isActiveStatus(s)).toBe(false);
    }
  });
});

describe("isPendingStatus", () => {
  test("covers the not-yet-started statuses", () => {
    for (const s of ["pending", "queued", "not_started"]) {
      expect(isPendingStatus(s)).toBe(true);
    }
  });

  test("excludes running and terminal statuses", () => {
    for (const s of ["in-progress", "running", "finished", "error"]) {
      expect(isPendingStatus(s)).toBe(false);
    }
  });
});

describe("isUnfinishedStatus", () => {
  test("is the union of active and pending", () => {
    for (const s of ["in-progress", "running", "pending", "queued", "not_started"]) {
      expect(isUnfinishedStatus(s)).toBe(true);
    }
    for (const s of ["finished", "completed", "error", "timed-out"]) {
      expect(isUnfinishedStatus(s)).toBe(false);
    }
  });
});

describe("statusTitle for the badge", () => {
  // What the step badge renders now that pending no longer hits the spinner.
  test("renders pending as Pending, not Running", () => {
    expect(statusTitle("pending")).toBe("Pending");
  });

  // Only the first character is capitalised (matches the existing
  // "Approval awaiting" badge), so pin that rather than title case.
  test("humanises hyphenated statuses with sentence case", () => {
    expect(statusTitle("in-progress")).toBe("In progress");
    expect(statusTitle("approval-awaiting")).toBe("Approval awaiting");
  });
});

describe("statusText", () => {
  test("does not call a queued step 'In progress'", () => {
    expect(statusText("pending")).toBe("Waiting");
    expect(statusText("queued")).toBe("Waiting");
    expect(statusText("not_started")).toBe("Waiting");
  });

  test("still reports genuinely running work as In progress", () => {
    expect(statusText("in-progress")).toBe("In progress");
    expect(statusText("running")).toBe("In progress");
  });

  test("leaves the other labels alone", () => {
    expect(statusText("approval-awaiting")).toBe("Awaiting approval");
    expect(statusText("completed")).toBe("Completed");
    expect(statusText("error")).toBe("Failed");
  });
});

// groupDerivedStatus deliberately uses the union, because polling keys off it.
// Splitting isActiveStatus must not change what this function returns.
describe("groupDerivedStatus is unaffected by the split", () => {
  test("a group whose only step is pending is still in-progress", () => {
    expect(groupDerivedStatus(group({ steps: [step({ status: "pending" })] }))).toBe(
      "in-progress",
    );
  });

  test("approval-awaiting still wins", () => {
    const g = group({
      steps: [
        step({ id: "a", execution_type: "approval", status: "approval-awaiting" }),
        step({ id: "b", execution_type: "system", status: "pending" }),
      ],
    });
    expect(groupDerivedStatus(g)).toBe("approval-awaiting");
  });

  test("an errored step still wins", () => {
    const g = group({
      steps: [
        step({ id: "a", execution_type: "approval", status: "error" }),
        step({ id: "b", execution_type: "system", status: "pending" }),
      ],
    });
    expect(groupDerivedStatus(g)).toBe("error");
  });

  test("all terminal-success steps complete the group", () => {
    const g = group({
      steps: [
        step({ id: "a", execution_type: "approval", status: "completed" }),
        step({ id: "b", execution_type: "system", status: "completed" }),
      ],
    });
    expect(groupDerivedStatus(g)).toBe("completed");
  });
});

// Polling continues while any group is non-terminal, which is what keeps the
// wizard live while a step sits queued behind an approval.
describe("aggregateGroupStatus keeps polling alive", () => {
  test("pending-only groups aggregate to in-progress", () => {
    expect(aggregateGroupStatus([group({ steps: [step({ status: "pending" })] })])).toBe(
      "in-progress",
    );
  });

  test("an awaiting-approval group aggregates to approval-awaiting", () => {
    const g = group({
      steps: [step({ execution_type: "approval", status: "approval-awaiting" })],
    });
    expect(aggregateGroupStatus([g])).toBe("approval-awaiting");
  });

  test("all-complete groups aggregate to completed", () => {
    const g = group({ steps: [step({ status: "completed" })] });
    expect(aggregateGroupStatus([g])).toBe("completed");
  });
});
