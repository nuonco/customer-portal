import { Badge } from "@/components/common/Badge";
import { Button } from "@/components/common/Button";
import { Card } from "@/components/common/Card";
import { Input } from "@/components/common/form/Input";
import { Link } from "@/components/common/Link";
import { Message } from "@/components/common/Message";
import { SimpleTable } from "@/components/common/SimpleTable";
import { Tabs } from "@/components/common/Tabs";
import { Text } from "@/components/common/Text";
import { ModalBase } from "@/components/surfaces/Modal";
import {
  formatInstallDate,
  formatInstallStatus,
  getInstallStatusBadgeTheme,
} from "@/utils/install-utils";
import { useMemo, useState } from "react";
import { useLocation } from "react-router";
import type { TCustomerInstallDetail } from "@/lib/api/customer/get-install-detail";
import { useSelectedInstall } from "./SelectedInstallProvider";

type TEditableInputField = {
  name: string;
  displayName: string;
  description: string;
  sensitive: boolean;
  groupName: string;
};

type TInstallInputsResponse = {
  inputs?: Record<string, string>;
  input_config?: {
    input_groups?: Array<{
      name?: string;
      display_name?: string;
      inputs?: Array<{
        name?: string;
        display_name?: string;
        description?: string;
        sensitive?: boolean;
      }>;
    }>;
    inputs?: Array<{
      name?: string;
      display_name?: string;
      description?: string;
      sensitive?: boolean;
    }>;
  };
};

function repoHref(repo: string): string {
  if (!repo) {
    return "";
  }

  if (repo.startsWith("http://") || repo.startsWith("https://")) {
    return repo;
  }

  return `https://github.com/${repo}`;
}

function awsConsoleAccountHref(): string {
  return "https://console.aws.amazon.com/console/home";
}

function awsConsoleRegionHref(region: string): string {
  return `https://console.aws.amazon.com/console/home?region=${encodeURIComponent(region)}`;
}

function repoBranchHref(repo: string, branch: string): string {
  const href = repoHref(repo);
  if (!href || !branch) {
    return "";
  }

  return `${href}/tree/${encodeURIComponent(branch)}`;
}

function extractEditableInputFields(
  payload: TInstallInputsResponse,
): TEditableInputField[] {
  const fields: TEditableInputField[] = [];
  const seen = new Set<string>();

  const pushField = (
    field: {
      name?: string;
      display_name?: string;
      description?: string;
      sensitive?: boolean;
    },
    groupName: string,
  ) => {
    const name = (field.name || "").trim();
    if (!name || seen.has(name)) {
      return;
    }

    seen.add(name);
    fields.push({
      name,
      displayName: field.display_name || name,
      description: field.description || "",
      sensitive: !!field.sensitive,
      groupName,
    });
  };

  const groups = payload.input_config?.input_groups || [];
  groups.forEach((group) => {
    const groupTitle = group.display_name || group.name || "Inputs";
    (group.inputs || []).forEach((field) => pushField(field, groupTitle));
  });

  (payload.input_config?.inputs || []).forEach((field) =>
    pushField(field, "Inputs"),
  );

  return fields;
}

function StackCard({
  installId,
  status,
  region,
  accountId,
}: {
  installId: string;
  status: string;
  region: string;
  accountId: string;
}) {
  return (
    <Card className="bg-surface">
      <div className="mb-3 flex items-center justify-between gap-2">
        <Link href={`/installs/${installId}/stack`} className="no-underline!">
          <Text as="h3" variant="h3" weight="stronger">
            Stack
          </Text>
        </Link>

        <Badge theme={getInstallStatusBadgeTheme(status)}>
          {formatInstallStatus(status)}
        </Badge>
      </div>

      <div className="flex gap-15">
        {Boolean(region) && (
          <div>
            <Text as="dt" variant="subtext" theme="neutral">
              Region
            </Text>
            <Text as="dd" variant="body">
              <Link href={awsConsoleRegionHref(region)} isATag isExternal>
                {region}
              </Link>
            </Text>
          </div>
        )}

        {Boolean(accountId) && (
          <div>
            <Text as="dt" variant="subtext" theme="neutral">
              Account ID
            </Text>
            <Text as="dd" variant="body">
              <Link href={awsConsoleAccountHref()} isATag isExternal>
                {accountId}
              </Link>
            </Text>
          </div>
        )}
      </div>
    </Card>
  );
}

function SandboxCard({
  installId,
  status,
  repo,
  branch,
  repoPublic,
}: {
  installId: string;
  status: string;
  repo: string;
  branch: string;
  repoPublic: boolean;
}) {
  const href = repoHref(repo);
  const branchHref = repoPublic ? repoBranchHref(repo, branch) : "";

  return (
    <Card className="bg-surface">
      <div className="mb-3 flex items-center justify-between gap-2">
        <Link href={`/installs/${installId}/sandbox`} className="no-underline!">
          <Text as="h3" variant="h3" weight="stronger">
            Sandbox
          </Text>
        </Link>

        <Badge theme={getInstallStatusBadgeTheme(status)}>
          {formatInstallStatus(status)}
        </Badge>
      </div>

      <div className="flex gap-15">
        {Boolean(href) && (
          <div>
            <Text as="dt" variant="subtext" theme="neutral">
              Repo
            </Text>
            <Text as="dd" variant="body">
              <Link href={href} isATag isExternal>
                {repo}
              </Link>
            </Text>
          </div>
        )}

        {Boolean(branch) && (
          <div>
            <Text as="dt" variant="subtext" theme="neutral">
              Branch
            </Text>
            <Text as="dd" variant="body">
              <Link href={branchHref} isATag isExternal>
                {branch}
              </Link>
            </Text>
          </div>
        )}
      </div>
    </Card>
  );
}

function HistoryTable({
  workflows,
}: {
  workflows: TCustomerInstallDetail["overview"]["recent_workflows"];
}) {
  if (!workflows || workflows.length === 0) {
    return (
      <Text variant="body" theme="neutral">
        No workflows recorded yet.
      </Text>
    );
  }

  return (
    <SimpleTable
      headers={[
        { key: "name", label: "Workflow" },
        { key: "status", label: "Status" },
        { key: "created", label: "Created" },
      ]}
    >
      {workflows.map((workflow) => (
        <tr key={workflow.id}>
          <td className="px-4 py-3">
            <Text variant="body">{workflow.name}</Text>
          </td>
          <td className="px-4 py-3">
            <Badge theme={getInstallStatusBadgeTheme(workflow.status)}>
              {formatInstallStatus(workflow.status)}
            </Badge>
          </td>
          <td className="px-4 py-3">
            <Text variant="body">{formatInstallDate(workflow.created_at)}</Text>
          </td>
        </tr>
      ))}
    </SimpleTable>
  );
}

function InputsTable({
  inputs,
}: {
  inputs: TCustomerInstallDetail["overview"]["input_fields"];
}) {
  if (!inputs || inputs.length === 0) {
    return (
      <Text variant="body" theme="neutral">
        No input values available.
      </Text>
    );
  }

  return (
    <SimpleTable
      headers={[
        { key: "name", label: "Input" },
        { key: "value", label: "Value" },
      ]}
    >
      {inputs.map((input) => (
        <tr key={input.name}>
          <td className="px-4 py-3">
            <Text as="div" variant="body" weight="stronger">
              {input.display_name || input.name}
            </Text>
            <Text as="div" variant="subtext" theme="neutral">
              {input.name}
            </Text>
          </td>
          <td className="px-4 py-3">
            <Text variant="body">
              {input.sensitive ? "••••••••" : input.value || "-"}
            </Text>
          </td>
        </tr>
      ))}
    </SimpleTable>
  );
}

/**
 * Reads the server's alert message out of a JSON response.
 *
 * These endpoints reply { status, message } — the message carries the real cause
 * (e.g. what the Nuon API said), so surfacing only the HTTP status would throw it
 * away and report "(500)" instead.
 */
async function alertMessage(
  response: Response,
  fallback: string,
): Promise<string> {
  try {
    const body = (await response.json()) as { message?: string };
    return body.message || fallback;
  } catch {
    return fallback;
  }
}

export const CustomerInstallOverviewView = () => {
  const { appName, install, installDetail, legacyBasePath } =
    useSelectedInstall();
  const location = useLocation();
  const overviewTab = new URLSearchParams(location.search).get("overview_tab");
  const initialTab = overviewTab === "inputs" ? "inputs" : "history";

  const [isEditInputsOpen, setIsEditInputsOpen] = useState(false);
  const [isReprovisionOpen, setIsReprovisionOpen] = useState(false);
  const [isDeprovisionOpen, setIsDeprovisionOpen] = useState(false);
  const [isLoadingInputs, setIsLoadingInputs] = useState(false);
  const [isSubmittingInputs, setIsSubmittingInputs] = useState(false);
  const [isSubmittingReprovision, setIsSubmittingReprovision] = useState(false);
  const [isSubmittingDeprovision, setIsSubmittingDeprovision] = useState(false);
  const [editInputsError, setEditInputsError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);
  const [editableFields, setEditableFields] = useState<TEditableInputField[]>(
    [],
  );
  const [editableValues, setEditableValues] = useState<Record<string, string>>(
    {},
  );

  const stack = installDetail.overview?.stack ?? {
    status: "",
    region: "",
    account_id: "",
  };
  const sandbox = installDetail.overview?.sandbox ?? {
    status: "",
    repo: "",
    branch: "",
    repo_public: false,
  };
  const workflows = installDetail.overview?.recent_workflows ?? [];
  const inputs = installDetail.overview?.input_fields ?? [];

  const groupedEditableFields = useMemo(() => {
    const groups = new Map<string, TEditableInputField[]>();
    editableFields.forEach((field) => {
      if (!groups.has(field.groupName)) {
        groups.set(field.groupName, []);
      }
      groups.get(field.groupName)?.push(field);
    });
    return Array.from(groups.entries());
  }, [editableFields]);

  const loadEditableInputs = async () => {
    setIsLoadingInputs(true);
    setEditInputsError(null);
    setEditableFields([]);
    setEditableValues({});

    try {
      const response = await fetch(`${legacyBasePath}/inputs`, {
        credentials: "include",
        headers: { Accept: "application/json" },
      });

      if (!response.ok) {
        throw new Error(`Failed to load inputs (${response.status})`);
      }

      const payload = (await response.json()) as TInstallInputsResponse;
      const fields = extractEditableInputFields(payload);
      const values = payload.inputs || {};

      setEditableFields(fields);
      setEditableValues(values);
    } catch (error) {
      setEditInputsError(
        error instanceof Error
          ? error.message
          : "Failed to load install inputs.",
      );
    } finally {
      setIsLoadingInputs(false);
    }
  };

  const openEditInputsModal = () => {
    setActionError(null);
    setActionSuccess(null);
    setIsEditInputsOpen(true);
    void loadEditableInputs();
  };

  const submitEditInputs = async () => {
    setIsSubmittingInputs(true);
    setEditInputsError(null);

    try {
      const response = await fetch(`${legacyBasePath}/inputs`, {
        method: "PUT",
        credentials: "include",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ inputs: editableValues }),
      });

      if (!response.ok) {
        let errorText = `Failed to update inputs (${response.status})`;
        try {
          const data = (await response.json()) as { error?: string };
          if (data.error) {
            errorText = data.error;
          }
        } catch {
          // Keep fallback message when non-JSON error bodies are returned.
        }
        throw new Error(errorText);
      }

      setIsEditInputsOpen(false);
      setActionSuccess(
        "Inputs updated successfully. A new workflow has been triggered.",
      );
    } catch (error) {
      setEditInputsError(
        error instanceof Error ? error.message : "Failed to update inputs.",
      );
    } finally {
      setIsSubmittingInputs(false);
    }
  };

  const submitReprovision = async () => {
    setIsSubmittingReprovision(true);
    setActionError(null);
    setActionSuccess(null);

    try {
      const response = await fetch(`${legacyBasePath}/reprovision`, {
        method: "POST",
        credentials: "include",
        headers: { Accept: "application/json" },
      });

      if (!response.ok) {
        throw new Error(
          await alertMessage(response, "Failed to reprovision install."),
        );
      }

      setIsReprovisionOpen(false);
      setActionSuccess(
        await alertMessage(
          response,
          "Install reprovisioning initiated successfully.",
        ),
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "Failed to reprovision install.",
      );
    } finally {
      setIsSubmittingReprovision(false);
    }
  };

  const submitDeprovision = async () => {
    setIsSubmittingDeprovision(true);
    setActionError(null);
    setActionSuccess(null);

    try {
      const response = await fetch(`${legacyBasePath}/deprovision`, {
        method: "POST",
        credentials: "include",
        headers: { Accept: "application/json" },
      });

      if (!response.ok) {
        throw new Error(
          await alertMessage(response, "Failed to deprovision install."),
        );
      }

      setIsDeprovisionOpen(false);
      setActionSuccess(
        await alertMessage(
          response,
          "Install deprovisioning initiated successfully.",
        ),
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "Failed to deprovision install.",
      );
    } finally {
      setIsSubmittingDeprovision(false);
    }
  };

  return (
    <div className="flex flex-col gap-5">
      {actionSuccess ? <Message theme="success">{actionSuccess}</Message> : null}
      {actionError ? <Message theme="warning">{actionError}</Message> : null}

      <div className="flex justify-between gap-10">
        <div className="flex items-center gap-4">
          <Text
            as="h1"
            variant="h2"
            weight="stronger"
            className="whitespace-nowrap"
          >
            {install.name || "-"}
          </Text>

          <Badge
            theme={getInstallStatusBadgeTheme(install.status)}
            className="shrink-0"
          >
            {formatInstallStatus(install.status)}
          </Badge>
        </div>
        <div className=" w-full flex flex-wrap justify-end gap-2">
          <Button onClick={openEditInputsModal} variant="secondary">
            Edit Inputs
          </Button>
          <Button
            onClick={() => setIsReprovisionOpen(true)}
            variant="secondary"
          >
            Reprovision
          </Button>
          <Button onClick={() => setIsDeprovisionOpen(true)} variant="danger">
            Deprovision
          </Button>
        </div>
      </div>

      <Card className="bg-surface">
        <dl className="flex flex-wrap items-center gap-15">
          <div>
            <Text as="dt" variant="subtext" theme="neutral">
              App
            </Text>
            <Text as="dd" variant="body">
              {appName || "-"}
            </Text>
          </div>

          <div>
            <Text as="dt" variant="subtext" theme="neutral">
              Created
            </Text>
            <Text as="dd" variant="body">
              {formatInstallDate(install.created_at)}
            </Text>
          </div>
          <div>
            <Text as="dt" variant="subtext" theme="neutral">
              Visibility
            </Text>
            <Text as="dd" variant="body">
              {install.visibility === "account"
                ? "Shared with current group"
                : "Private"}
            </Text>
          </div>
        </dl>
      </Card>

      <div className="grid gap-4 md:grid-cols-2">
        <StackCard
          installId={install.id}
          status={stack.status}
          region={stack.region}
          accountId={stack.account_id}
        />
        <SandboxCard
          installId={install.id}
          status={sandbox.status}
          repo={sandbox.repo}
          branch={sandbox.branch}
          repoPublic={sandbox.repo_public}
        />
      </div>

      <Card className="bg-surface">
        <Text as="h3" variant="h3" weight="stronger" className="mb-2">
          Overview Data
        </Text>
        <Tabs
          initActiveTab={initialTab}
          tabs={{
            history: (
              <div className="pt-4">
                <HistoryTable workflows={workflows} />
              </div>
            ),
            inputs: (
              <div className="pt-4">
                <InputsTable inputs={inputs} />
              </div>
            ),
          }}
          tabsClassName="pt-3"
        />
      </Card>

      <ModalBase
        isVisible={isEditInputsOpen}
        onClose={() => setIsEditInputsOpen(false)}
        heading="Edit Inputs"
        showFooter={false}
        size="lg"
      >
        {isLoadingInputs ? (
          <Text variant="body" theme="neutral">
            Loading inputs...
          </Text>
        ) : editInputsError ? (
          <Message theme="warning">{editInputsError}</Message>
        ) : editableFields.length === 0 ? (
          <Text variant="body" theme="neutral">
            No editable inputs are available for this install.
          </Text>
        ) : (
          <form
            className="space-y-5"
            onSubmit={(event) => {
              event.preventDefault();
              void submitEditInputs();
            }}
          >
            {groupedEditableFields.map(([groupName, fields]) => (
              <div key={groupName} className="space-y-4">
                <Text as="div" variant="body" weight="stronger">
                  {groupName}
                </Text>
                {fields.map((field) => (
                  <div key={field.name} className="space-y-1">
                    <Input
                      id={`edit-input-${field.name}`}
                      value={editableValues[field.name] ?? ""}
                      type={field.sensitive ? "password" : "text"}
                      onChange={(event) =>
                        setEditableValues((prev) => ({
                          ...prev,
                          [field.name]: event.target.value,
                        }))
                      }
                      labelProps={{ labelText: field.displayName }}
                    />
                    {field.description ? (
                      <Text as="div" variant="subtext" theme="neutral">
                        {field.description}
                      </Text>
                    ) : null}
                  </div>
                ))}
              </div>
            ))}

            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={() => setIsEditInputsOpen(false)}
              >
                Cancel
              </Button>
              <Button type="submit" size="sm" disabled={isSubmittingInputs}>
                {isSubmittingInputs ? "Updating..." : "Update Inputs"}
              </Button>
            </div>
          </form>
        )}
      </ModalBase>

      <ModalBase
        isVisible={isReprovisionOpen}
        onClose={() => setIsReprovisionOpen(false)}
        heading="Reprovision Installation?"
        showFooter={false}
        size="sm"
      >
        <div className="space-y-5">
          <Text variant="body" theme="neutral">
            This will re-run the provisioning workflow for this installation.
            Existing resources will be updated in place.
          </Text>
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => setIsReprovisionOpen(false)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              size="sm"
              disabled={isSubmittingReprovision}
              onClick={() => void submitReprovision()}
            >
              {isSubmittingReprovision ? "Reprovisioning..." : "Reprovision"}
            </Button>
          </div>
        </div>
      </ModalBase>

      <ModalBase
        isVisible={isDeprovisionOpen}
        onClose={() => setIsDeprovisionOpen(false)}
        heading="Deprovision Installation?"
        showFooter={false}
        size="sm"
      >
        <div className="space-y-5">
          <Text variant="body" theme="neutral">
            Are you sure you want to deprovision this installation? This action
            cannot be undone and will destroy all data associated with this
            installation.
          </Text>
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => setIsDeprovisionOpen(false)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant="danger"
              size="sm"
              disabled={isSubmittingDeprovision}
              onClick={() => void submitDeprovision()}
            >
              {isSubmittingDeprovision ? "Deprovisioning..." : "Deprovision"}
            </Button>
          </div>
        </div>
      </ModalBase>
    </div>
  );
};
