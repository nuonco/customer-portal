import { inferCustomerSubdomain } from "@/lib/runtime-config";
export type TCustomerWizardStep = "inputs" | "stack" | "sandbox" | "components";

export type TCustomerWizardApp = {
  app_id: string;
  display_name: string;
  summary: string;
  status: string;
  logo_light: string;
  logo_dark: string;
  platform: string;
};

export type TCustomerWizardInputField = {
  name: string;
  display_name: string;
  description: string;
  type: string;
  default: string;
  required: boolean;
  sensitive: boolean;
  index: number;
};

export type TCustomerWizardInputGroup = {
  name: string;
  display_name: string;
  description: string;
  inputs: TCustomerWizardInputField[];
};

export type TCustomerWizardForm = {
  install_name?: string;
  platform: string;
  input_groups: TCustomerWizardInputGroup[];
};

export type TCustomerWizardApproval = {
  step_id: string;
  approval_id: string;
  type: string;
};

export type TCustomerWizardPlanSummary = {
  kind: string;
  create_count: number;
  update_count: number;
  delete_count: number;
  replace_count: number;
  add_count: number;
  change_count: number;
  destroy_count: number;
  total_changes: number;
};

export type TCustomerWizardGroupStep = {
  id: string;
  name: string;
  execution_type: string;
  status: string;
  status_human_description?: string;
  retryable: boolean;
  finished: boolean;
};

export type TCustomerWizardGroup = {
  id: string;
  title: string;
  domain: string;
  status: string;
  active_step_name?: string;
  active_step_description?: string;
  can_approve: boolean;
  can_retry: boolean;
  approval?: TCustomerWizardApproval;
  retry_step_id?: string;
  plan_summary?: TCustomerWizardPlanSummary;
  steps: TCustomerWizardGroupStep[];
  image_url?: string;
  image_tag?: string;
  target_status?: string;
  target_description?: string;
};

export type TCustomerWizardWorkflow = {
  install: {
    id: string;
    name: string;
    status: string;
    region: string;
    created_at: string;
  };
  groups: TCustomerWizardGroup[];
  policy_totals: { pass: number; warn: number; deny: number };
  plan_summary?: TCustomerWizardPlanSummary;
  stack_setup?: {
    platform: string;
    cloudformation_link?: string;
    template_url?: string;
    stack_name?: string;
    region?: string;
    tfvars_content?: string;
    nuon_install_id?: string;
    azure_template_url?: string;
    azure_location?: string;
  };
  has_approval_awaiting: boolean;
  is_step_complete: boolean;
  is_step_error: boolean;
  next_step?: string;
  overview_path?: string;
};

export type TCustomerWizardState = {
  app: TCustomerWizardApp;
  form?: TCustomerWizardForm;
  workflow?: TCustomerWizardWorkflow;
  workflow_id?: string;
};

export type TCustomerWizardCreatePayload = {
  name: string;
  region?: string;
  location?: string;
  inputs: Record<string, string>;
};

export type TCustomerWizardCreateResponse = {
  install_id: string;
  workflow_id?: string;
  current_step: string;
  next_url: string;
};

type TCustomerWizardStateOptions = {
  appId: string;
  step?: string;
  installId?: string;
  workflowId?: string;
};

function buildWizardURL(pathname: string, params: Record<string, string | undefined>) {
  const search = new URLSearchParams();
  const subdomain = inferCustomerSubdomain();
  if (subdomain) {
    search.set("subdomain", subdomain);
  }
  Object.entries(params).forEach(([key, value]) => {
    if (value) {
      search.set(key, value);
    }
  });
  return `${pathname}${search.toString() ? `?${search.toString()}` : ""}`;
}

export async function getCustomerInstallWizardState({
  appId,
  step,
  installId,
  workflowId,
}: TCustomerWizardStateOptions): Promise<TCustomerWizardState> {
  const response = await fetch(
    buildWizardURL(`/portal-api/apps/${appId}/install-wizard`, {
      step,
      install_id: installId,
      workflow_id: workflowId,
    }),
    {
      cache: "no-store",
      credentials: "include",
      headers: { Accept: "application/json" },
    },
  );

  if (!response.ok) {
    throw new Error(`Customer install wizard request failed with status ${response.status}`);
  }

  return (await response.json()) as TCustomerWizardState;
}

export async function createCustomerInstallWizardInstall(
  appId: string,
  payload: TCustomerWizardCreatePayload,
): Promise<TCustomerWizardCreateResponse> {
  const response = await fetch(
    buildWizardURL(`/portal-api/apps/${appId}/install-wizard`, {}),
    {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify(payload),
    },
  );

  if (!response.ok) {
    let message = `Customer install creation failed with status ${response.status}`;
    try {
      const data = (await response.json()) as { error?: string };
      if (data.error) {
        message = data.error;
      }
    } catch {
      // noop
    }
    throw new Error(message);
  }

  return (await response.json()) as TCustomerWizardCreateResponse;
}
