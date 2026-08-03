import { expect, mock, test } from "bun:test";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { ConfigureStep } from "./steps/Configure";
import { getStepTransitionDirection } from "./InstallWizard";
import type { TCustomerWizardApp, TCustomerWizardForm } from "@/lib/api/customer/install-wizard";

const app: TCustomerWizardApp = {
  app_id: "app-payments",
  display_name: "Payments",
  summary: "Install payments stack",
  status: "published",
  logo_light: "",
  logo_dark: "",
  platform: "aws",
};

const form: TCustomerWizardForm = {
  platform: "aws",
  input_groups: [],
};

test("confirm button calls onConfirm from confirmation step", () => {
  const onConfirm = mock((_values) => {});

  render(
    <MemoryRouter>
      <ConfigureStep
        app={app}
        form={form}
        initialValues={{
          installName: "my-install",
          region: "us-east-1",
          location: "",
          inputs: {},
        }}
        error={null}
        isCreating={false}
        onConfirm={onConfirm}
      />
    </MemoryRouter>,
  );

  fireEvent.click(
    screen.getByRole("button", { name: /review and start provisioning/i }),
  );

  const confirmButton = screen.getByRole("button", { name: /confirm/i });
  expect(confirmButton).toBeInTheDocument();
  expect(onConfirm).not.toHaveBeenCalled();

  fireEvent.click(confirmButton);
  expect(onConfirm).toHaveBeenCalledTimes(1);
  expect(onConfirm.mock.calls[0][0]).toEqual({
    installName: "my-install",
    region: "us-east-1",
    location: "",
    inputs: {},
  });
});

test("requires install name before entering confirmation step", () => {
  const onConfirm = mock((_values) => {});

  render(
    <MemoryRouter>
      <ConfigureStep
        app={app}
        form={form}
        initialValues={{
          installName: "",
          region: "us-east-1",
          location: "",
          inputs: {},
        }}
        error={null}
        isCreating={false}
        onConfirm={onConfirm}
      />
    </MemoryRouter>,
  );

  fireEvent.click(
    screen.getByRole("button", { name: /review and start provisioning/i }),
  );

  expect(screen.queryByRole("button", { name: /confirm/i })).not.toBeInTheDocument();
  expect(screen.getByLabelText(/install name/i)).toHaveAttribute("aria-invalid", "true");
  expect(onConfirm).not.toHaveBeenCalled();
});

test("shows summary view in read-only mode", () => {
  const onConfirm = mock((_values) => {});

  render(
    <MemoryRouter>
      <ConfigureStep
        app={app}
        form={form}
        initialValues={{
          installName: "my-install",
          region: "us-east-1",
          location: "",
          inputs: {},
        }}
        error={null}
        isCreating={false}
        isReadOnly
        onConfirm={onConfirm}
      />
    </MemoryRouter>,
  );

  expect(screen.getByText(/configuration is read-only/i)).toBeInTheDocument();

  expect(screen.getByText(/install configuration/i)).toBeInTheDocument();
  expect(screen.getByText("my-install")).toBeInTheDocument();
  expect(screen.getByText(/us east/i)).toBeInTheDocument();
  expect(screen.queryByLabelText(/install name/i)).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: /confirm and deploy stack/i }),
  ).not.toBeInTheDocument();

  expect(onConfirm).not.toHaveBeenCalled();
});

test("returns forward/backward/none transition directions for wizard steps", () => {
  expect(getStepTransitionDirection("inputs", "stack")).toBe("forward");
  expect(getStepTransitionDirection("components", "sandbox")).toBe(
    "backward",
  );
  expect(getStepTransitionDirection("sandbox", "sandbox")).toBe("none");
});
