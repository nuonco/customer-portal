import { useForm } from "@tanstack/react-form";
import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/common/Button";
import { Card } from "@/components/common/Card";
import { Icon } from "@/components/common/Icon";
import { Loading } from "@/components/common/Loading";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { Input } from "@/components/common/form/Input";
import { Select } from "@/components/common/form/Select";
import { Textarea } from "@/components/common/form/Textarea";
import { AWS_REGIONS, AZURE_REGIONS } from "@/configs/cloud-regions";
import type {
  TCustomerWizardApp,
  TCustomerWizardForm,
} from "@/lib/api/customer/install-wizard";
import { getFlagEmoji } from "@/utils/string-utils";
import { useInstallWizard } from "@/hooks/use-install-wizard";
import {
  buildInitialWizardValues,
  isBooleanField,
  type TInstallWizardFormValues,
} from "../wizard-utils";
import { useSearchParams } from "react-router";

const regions = AWS_REGIONS.map((region) => ({
  value: region.value,
  label: region?.iconVariant
    ? `${getFlagEmoji(region.iconVariant.substring(5))} ${region.text} [${region.value}]`
    : region.text,
}));

const locations = AZURE_REGIONS.map((region) => ({
  value: region.value,
  label: region?.iconVariant
    ? `${getFlagEmoji(region.iconVariant.substring(5))} ${region.text}`
    : region.text,
}));

function formatCloudRegionLabel(
  value: string,
  options: Array<{ value: string; text: string; iconVariant?: string }>,
): string {
  const match = options.find((option) => option.value === value);
  if (!match) {
    return value;
  }

  const emoji = match.iconVariant
    ? `${getFlagEmoji(match.iconVariant.substring(5))} `
    : "";

  return `${emoji}${match.text} [${match.value}]`;
}

export function ConfigureStep({
  app,
  form,
  initialValues,
  error,
  isCreating,
  isReadOnly = false,
  onConfirm,
}: {
  app: TCustomerWizardApp;
  form: TCustomerWizardForm;
  initialValues: TInstallWizardFormValues;
  error: string | null;
  isCreating: boolean;
  isReadOnly?: boolean;
  onConfirm: (values: TInstallWizardFormValues) => void;
}) {
  const [searchParams, setSearchParams] = useSearchParams();
  const inputGroups = Array.isArray(form.input_groups) ? form.input_groups : [];

  const [isConfirming, setIsConfirming] = useState(false);
  const [clientError, setClientError] = useState<string | null>(null);
  const [confirmValues, setConfirmValues] =
    useState<TInstallWizardFormValues>(initialValues);

  const wizardForm = useForm({
    defaultValues: initialValues as TInstallWizardFormValues,
  });

  const summaryValues = isReadOnly ? wizardForm.state.values : confirmValues;

  const confirmRegionLabel =
    form.platform === "aws" && summaryValues.region
      ? formatCloudRegionLabel(summaryValues.region, AWS_REGIONS)
      : null;
  const confirmLocationLabel =
    form.platform === "azure" && summaryValues.location
      ? formatCloudRegionLabel(summaryValues.location, AZURE_REGIONS)
      : null;

  useEffect(() => {
    const initialInstallName = initialValues.installName.trim();
    const currentInstallName = searchParams.get("install_name")?.trim() ?? "";

    if (!initialInstallName || currentInstallName === initialInstallName) {
      return;
    }

    const next = new URLSearchParams(searchParams);
    next.set("install_name", initialInstallName);
    setSearchParams(next, { replace: true });
  }, [initialValues.installName, searchParams, setSearchParams]);

  const handlePrepareConfirm = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setClientError(null);

    if (isReadOnly) {
      return;
    }

    const values = wizardForm.state.values;
    if (!values.installName.trim()) {
      setClientError("Install name is required.");
      return;
    }

    if (form.platform === "aws" && !values.region) {
      setClientError("Deployment region is required.");
      return;
    }

    if (form.platform === "azure" && !values.location) {
      setClientError("Deployment location is required.");
      return;
    }

    for (const group of inputGroups) {
      const inputs = Array.isArray(group.inputs) ? group.inputs : [];
      for (const input of inputs) {
        if (!input.required || isBooleanField(input)) {
          continue;
        }

        if (!(values.inputs[input.name] ?? "").trim()) {
          setClientError(`${input.display_name || input.name} is required.`);
          return;
        }
      }
    }

    setConfirmValues(values);
    setIsConfirming(true);
  };

  if (isCreating) {
    return (
      <Card className="bg-surface">
        <div className="flex flex-col items-center justify-center gap-4 py-10 text-center">
          <Loading variant="large" />
          <Text as="h1" variant="h3" weight="stronger">
            Preparing to install {app.display_name}
          </Text>
          <Text as="p" variant="body" theme="neutral">
            This may take a few moments.
          </Text>
        </div>
      </Card>
    );
  }

  if (isConfirming || isReadOnly) {
    return (
      <div className="space-y-4">
        {isReadOnly ? (
          <Message theme="info">
            This step is complete. Configuration is read-only.
          </Message>
        ) : null}

        <Card className="bg-surface">
          <div>
            <Text as="h2" variant="h3" weight="stronger">
              {isReadOnly
                ? "Install configuration"
                : "Review install configuration"}
            </Text>
            {!isReadOnly ? (
              <Text as="p" variant="body" theme="neutral">
                Review the configuration below and confirm to start provisioning
                your install.
              </Text>
            ) : null}
          </div>

          <dl className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
            <div>
              <Text as="dt" variant="subtext" theme="neutral">
                Install name
              </Text>
              <Text as="dd" variant="body">
                {summaryValues.installName}
              </Text>
            </div>
            {confirmRegionLabel ? (
              <div>
                <Text as="dt" variant="subtext" theme="neutral">
                  Region
                </Text>
                <Text as="dd" variant="body">
                  {confirmRegionLabel}
                </Text>
              </div>
            ) : null}
            {confirmLocationLabel ? (
              <div>
                <Text as="dt" variant="subtext" theme="neutral">
                  Location
                </Text>
                <Text as="dd" variant="body">
                  {confirmLocationLabel}
                </Text>
              </div>
            ) : null}
          </dl>
        </Card>

        {inputGroups.map((group) => {
          const groupInputs = Array.isArray(group.inputs) ? group.inputs : [];
          const visibleInputs = groupInputs.filter(
            (input) =>
              summaryValues.inputs[input.name] !== undefined &&
              summaryValues.inputs[input.name] !== "",
          );
          if (visibleInputs.length === 0) {
            return null;
          }
          return (
            <Card key={group.name} className="bg-surface">
              <Text as="h3" variant="h3" weight="stronger">
                {group.display_name || group.name || "Configuration"}
              </Text>
              <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
                {visibleInputs.map((input) => (
                  <div key={input.name}>
                    <Text as="dt" variant="subtext" theme="neutral">
                      {input.display_name || input.name}
                    </Text>
                    <Text as="dd" variant="body">
                      {input.sensitive
                        ? "••••••••"
                        : isBooleanField(input)
                          ? summaryValues.inputs[input.name] === "true"
                            ? "Yes"
                            : "No"
                          : summaryValues.inputs[input.name]}
                    </Text>
                  </div>
                ))}
              </div>
            </Card>
          );
        })}

        {clientError ? <Message theme="warning">{clientError}</Message> : null}
        {error ? <Message theme="warning">{error}</Message> : null}

        {!isReadOnly ? (
          <div className="flex justify-end gap-4">
            <Button
              type="button"
              variant="ghost"
              size="md"
              onClick={() => {
                setClientError(null);
                setIsConfirming(false);
              }}
            >
              Go back
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              disabled={isCreating}
              onClick={() => onConfirm(confirmValues)}
            >
              Confirm and deploy stack
              <Icon variant="ArrowRightIcon" size={16} weight="bold" />
            </Button>
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <form className="space-y-4" onSubmit={handlePrepareConfirm}>
      <Card className="bg-surface">
        <div className="grid gap-x-8 gap-y-6 xl:grid-cols-2">
          <wizardForm.Field name="installName">
            {(field) => (
              <Input
                id="customer-install-name"
                value={field.state.value}
                onChange={(event) => {
                  const nextValue = event.target.value;
                  field.handleChange(nextValue);

                  const next = new URLSearchParams(searchParams);
                  if (nextValue.trim()) {
                    next.set("install_name", nextValue.trim());
                  } else {
                    next.delete("install_name");
                  }
                  setSearchParams(next, { replace: true });
                }}
                labelProps={{ labelText: "Install name" }}
                placeholder="my-install"
                disabled={isReadOnly}
                required
              />
            )}
          </wizardForm.Field>

          {form.platform === "aws" ? (
            <wizardForm.Field name="region">
              {(field) => (
                <Select
                  name="region"
                  options={regions}
                  value={field.state.value}
                  onChange={(event) => field.handleChange(event.target.value)}
                  labelProps={{ labelText: "Deployment region" }}
                  disabled={isReadOnly}
                  required
                />
              )}
            </wizardForm.Field>
          ) : null}

          {form.platform === "azure" ? (
            <wizardForm.Field name="location">
              {(field) => (
                <Select
                  name="location"
                  options={locations}
                  value={field.state.value}
                  onChange={(event) => field.handleChange(event.target.value)}
                  labelProps={{ labelText: "Deployment location" }}
                  disabled={isReadOnly}
                  required
                />
              )}
            </wizardForm.Field>
          ) : null}
        </div>
      </Card>

      {inputGroups.map((group) => (
        <Card key={group.name} className="bg-surface">
          <div>
            {Boolean(group.display_name || group.name) && (
              <Text as="h3" variant="h3" weight="stronger">
                {group.display_name || group.name}
              </Text>
            )}

            {Boolean(group.description) && (
              <Text as="p" variant="body" theme="neutral">
                {group.description}
              </Text>
            )}
          </div>
          <div className="grid gap-y-6">
            {(Array.isArray(group.inputs) ? group.inputs : []).map((input) => {
              if (isBooleanField(input)) {
                return (
                  <div className="col-span-full" key={input.name}>
                    <wizardForm.Field key={input.name} name="inputs">
                      {(field) => (
                        <label className="flex items-start gap-3 rounded-xl border border-border-subtle px-4 py-3">
                          <input
                            type="checkbox"
                            checked={field.state.value[input.name] === "true"}
                            disabled={isReadOnly}
                            onChange={(event) =>
                              field.handleChange((current) => ({
                                ...current,
                                [input.name]: event.target.checked
                                  ? "true"
                                  : "false",
                              }))
                            }
                            className="mt-1"
                          />
                          <div>
                            <Text as="div" variant="body" weight="stronger">
                              {input.display_name || input.name}
                            </Text>
                            {input.description ? (
                              <Text as="div" variant="subtext" theme="neutral">
                                {input.description}
                              </Text>
                            ) : null}
                          </div>
                        </label>
                      )}
                    </wizardForm.Field>
                  </div>
                );
              }

              if (input.type === "json") {
                return (
                  <wizardForm.Field key={input.name} name="inputs">
                    {(field) => (
                      <Textarea
                        id={`customer-install-${input.name}`}
                        value={field.state.value[input.name] ?? ""}
                        onChange={(event) =>
                          field.handleChange((current) => ({
                            ...current,
                            [input.name]: event.target.value,
                          }))
                        }
                        labelProps={{
                          labelText: input.display_name || input.name,
                        }}
                        helperText={input.description}
                        required={input.required}
                        autoResize
                        minRows={4}
                        disabled={isReadOnly}
                      />
                    )}
                  </wizardForm.Field>
                );
              }

              return (
                <wizardForm.Field key={input.name} name="inputs">
                  {(field) => (
                    <Input
                      id={`customer-install-${input.name}`}
                      type={input.sensitive ? "password" : "text"}
                      value={field.state.value[input.name] ?? ""}
                      onChange={(event) =>
                        field.handleChange((current) => ({
                          ...current,
                          [input.name]: event.target.value,
                        }))
                      }
                      labelProps={{
                        labelText: input.display_name || input.name,
                      }}
                      helperText={input.description}
                      placeholder={input.default || undefined}
                      required={input.required}
                      disabled={isReadOnly}
                    />
                  )}
                </wizardForm.Field>
              );
            })}
          </div>
        </Card>
      ))}

      {clientError ? <Message theme="warning">{clientError}</Message> : null}
      {error ? <Message theme="warning">{error}</Message> : null}

      <div className="flex gap-4 justify-end">
        <Button
          href="/customer/apps"
          variant="ghost"
          size="md"
          onClick={() => {
            setClientError(null);
            wizardForm.reset();
          }}
        >
          Cancel
        </Button>
        <Button type="submit" variant="primary" size="md" disabled={isReadOnly}>
          {isReadOnly ? (
            "Completed"
          ) : (
            <>
              Review and start provisioning
              <Icon variant="ArrowRightIcon" size={16} weight="bold" />
            </>
          )}
        </Button>
      </div>
    </form>
  );
}

export function ConfigureStepFromContext() {
  const {
    wizardState,
    currentStep,
    stepStates,
    createError,
    isCreating,
    confirmInstall,
  } = useInstallWizard();

  if (!wizardState?.form) {
    return null;
  }

  const initialValues = buildInitialWizardValues(wizardState.form);
  const wizardFormKey = JSON.stringify(initialValues);

  return (
    <ConfigureStep
      key={wizardFormKey}
      app={wizardState.app}
      form={wizardState.form}
      initialValues={initialValues}
      error={createError}
      isCreating={isCreating}
      isReadOnly={
        stepStates[currentStep]?.status === "completed" ||
        stepStates[currentStep]?.status === "success"
      }
      onConfirm={confirmInstall}
    />
  );
}
