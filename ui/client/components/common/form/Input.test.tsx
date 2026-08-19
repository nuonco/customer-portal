import { beforeEach, expect, test } from "bun:test";
import { useRef, useState } from "react";
import { GlobalRegistrator } from "@happy-dom/global-registrator";
import { fireEvent, render, screen } from "@testing-library/react";

import { Input } from "./Input";

// Several test files in this suite replace globalThis.window wholesale with a
// stub (`Object.defineProperty(globalThis, "window", { value: { location } })`)
// to fake a hostname. bun shares one process across files, so that stub survives
// into whichever file runs next and leaves rendering tests without a real DOM.
// Restore a genuine happy-dom environment before each test here.
beforeEach(() => {
  const w = globalThis.window as unknown as { document?: unknown } | undefined;
  if (!w?.document) {
    GlobalRegistrator.register();
  }
  document.body.innerHTML = "";
});

const MSG = "Please fill out this field";
const LABEL = "Nuon Org ID";

// Mirrors how ConnectOrgModal uses it: controlled and required.
function Harness({
  required = true,
  ...rest
}: {
  required?: boolean;
  error?: boolean;
  errorMessage?: string;
  helperText?: string;
}) {
  const [value, setValue] = useState("");
  return (
    <form>
      <Input
        id="org-id"
        labelProps={{ labelText: LABEL }}
        required={required}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        {...rest}
      />
    </form>
  );
}

function getInput() {
  return screen.getByLabelText(LABEL) as HTMLInputElement;
}

test("stays quiet until the user has interacted", () => {
  render(<Harness />);
  expect(screen.queryByText(MSG)).toBeNull();
});

test("shows the required message on blur while empty", () => {
  render(<Harness />);
  fireEvent.blur(getInput());
  expect(screen.queryByText(MSG)).not.toBeNull();
});

// The reported bug: select-all + backspace, then refill, left the message
// stranded over a filled field.
test("clears the required message once the field is refilled", () => {
  render(<Harness />);
  const input = getInput();

  fireEvent.change(input, { target: { value: "org_abc123" } });
  fireEvent.blur(input);
  expect(screen.queryByText(MSG)).toBeNull();

  fireEvent.change(input, { target: { value: "" } });
  fireEvent.blur(input);
  expect(screen.queryByText(MSG)).not.toBeNull();

  fireEvent.change(input, { target: { value: "org_abc123" } });

  expect(input.value).toBe("org_abc123");
  expect(screen.queryByText(MSG)).toBeNull();
});

// Root cause: validity used to be re-synced only from a native `input` event, so
// any value change that did not produce one left the message behind. This is the
// case that must keep working.
test("clears the message when the value changes without a native input event", () => {
  function ProgrammaticHarness() {
    const [value, setValue] = useState("");
    return (
      <form>
        <Input
          id="org-id"
          labelProps={{ labelText: LABEL }}
          required
          value={value}
          onChange={(e) => setValue(e.target.value)}
        />
        <button type="button" onClick={() => setValue("org_from_state")}>
          fill
        </button>
      </form>
    );
  }

  render(<ProgrammaticHarness />);
  fireEvent.blur(getInput());
  expect(screen.queryByText(MSG)).not.toBeNull();

  // No keystroke, no native input event — only a React state update.
  fireEvent.click(screen.getByText("fill"));

  expect(getInput().value).toBe("org_from_state");
  expect(screen.queryByText(MSG)).toBeNull();
});

test("a submit attempt on an empty field reveals the message", () => {
  render(<Harness />);
  const input = getInput();

  // The browser fires `invalid` on a failed constraint check at submit time.
  fireEvent.invalid(input);

  expect(screen.queryByText(MSG)).not.toBeNull();
});

test("never shows the required message when not required", () => {
  render(<Harness required={false} />);
  fireEvent.blur(getInput());
  expect(screen.queryByText(MSG)).toBeNull();
});

test("a parent-supplied error takes precedence over helper text", () => {
  render(
    <Harness error errorMessage="Org already connected" helperText="Some hint" />,
  );
  expect(screen.queryByText("Org already connected")).not.toBeNull();
  expect(screen.queryByText("Some hint")).toBeNull();
  expect(getInput().getAttribute("aria-invalid")).toBe("true");
});

test("shows helper text when there is nothing wrong", () => {
  render(<Harness helperText="Some hint" />);
  expect(screen.queryByText("Some hint")).not.toBeNull();
  expect(getInput().getAttribute("aria-invalid")).toBe("false");
});

// ConnectOrgModal forwards a ref to autofocus this field on open.
test("forwards the ref to the underlying input", () => {
  let captured: HTMLInputElement | null = null;

  function RefHarness() {
    const ref = useRef<HTMLInputElement | null>(null);
    return (
      <Input
        id="org-id"
        labelProps={{ labelText: LABEL }}
        value=""
        onChange={() => {}}
        ref={(node) => {
          ref.current = node;
          captured = node;
        }}
      />
    );
  }

  render(<RefHarness />);
  expect(captured).not.toBeNull();
  expect(captured).toBe(getInput());
});
