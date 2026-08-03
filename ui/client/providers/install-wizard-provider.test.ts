import { expect, test } from "bun:test";
import {
  createWizardStepStates,
  mergeWizardStepStates,
} from "./install-wizard-provider";

test("preserves reached step state when navigating backward", () => {
  const stackStepStates = createWizardStepStates("stack", "inst_1");
  const inputsStepStates = createWizardStepStates("inputs", "inst_1");

  const merged = mergeWizardStepStates(stackStepStates, inputsStepStates);

  expect(merged.inputs.status).toBe("completed");
  expect(merged.stack.status).toBe("in-progress");
  expect(merged.sandbox.status).toBe("upcoming");
  expect(merged.components.status).toBe("upcoming");
});

test("advances previously reached steps as progress continues", () => {
  const stackStepStates = createWizardStepStates("stack", "inst_1");
  const sandboxStepStates = createWizardStepStates("sandbox", "inst_1");

  const merged = mergeWizardStepStates(stackStepStates, sandboxStepStates);

  expect(merged.stack.status).toBe("completed");
  expect(merged.sandbox.status).toBe("in-progress");
});

test("keeps deploy stack in-progress when reloading configure for an existing install", () => {
  const inputsStepStates = createWizardStepStates("inputs", "inst_1");

  expect(inputsStepStates.inputs.status).toBe("completed");
  expect(inputsStepStates.stack.status).toBe("in-progress");
  expect(inputsStepStates.stack.accessible).toBe(true);
  expect(inputsStepStates.sandbox.status).toBe("upcoming");
  expect(inputsStepStates.components.status).toBe("upcoming");
});
