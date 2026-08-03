import { extractSubdomain } from "@/utils/subdomain-utils";

export type TCustomerInstallWorkflow = {
  id: string;
  name: string;
  status: string;
  created_at: string;
  finished_at: string;
};

export type TCustomerInstallDetail = {
  install: {
    id: string;
    name: string;
    status: string;
    visibility: string;
    created_at: string;
    region: string;
    app_id: string;
    app_name: string;
  };
  app: {
    display_name: string;
    summary: string;
    platform: string;
    logo_light: string;
    logo_dark: string;
  };
  api_deleted_error: boolean;
  nuon_api_error?: string;
  overview: {
    stack: {
      status: string;
      region: string;
      account_id: string;
    };
    sandbox: {
      status: string;
      repo: string;
      branch: string;
      repo_public: boolean;
    };
    components: {
      id?: string;
      name: string;
      type: string;
      status: string;
      repo: string;
      directory: string;
      branch: string;
      repo_public: boolean;
    }[];
    recent_workflows: TCustomerInstallWorkflow[];
    input_fields: {
      name: string;
      display_name: string;
      value: string;
      sensitive: boolean;
    }[];
  };
  stack: {
    status: string;
    region: string;
    account_id: string;
    vpc: string;
    outputs?: Record<string, string>;
    recent_runs: {
      id: string;
      status: string;
      version_status?: string;
      created_at: string;
      finished_at: string;
    }[];
  };
  sandbox: {
    status: string;
    repo: string;
    directory: string;
    branch: string;
    repo_public: boolean;
    outputs?: Record<string, string>;
    recent_runs: {
      id: string;
      status: string;
      created_at: string;
      finished_at: string;
    }[];
  };
  components: {
    items: {
      id?: string;
      name: string;
      type: string;
      status: string;
      repo: string;
      directory: string;
      branch: string;
      repo_public: boolean;
    }[];
    deploys: {
      id: string;
      component_id: string;
      status: string;
      created_at: string;
      finished_at: string;
    }[];
  };
  roles: {
    roles: {
      name: string;
      type: string;
      arn: string;
    }[];
  };
  policies: {
    totals: {
      pass: number;
      warn: number;
      deny: number;
    };
    items: {
      name: string;
      type: string;
      engine: string;
    }[];
    reports: {
      id: string;
      policy_name: string;
      pass_count: number;
      warn_count: number;
      deny_count: number;
      created_at: string;
    }[];
  };
  audit: {
    workflows: TCustomerInstallWorkflow[];
    action_workflows: TCustomerInstallWorkflow[];
  };
  readme: {
    markdown: string;
  };
  legacy_base_path: string;
};

function inferCustomerSubdomain(): string {
  if (typeof window === "undefined") {
    return "";
  }

  const configuredBaseDomain =
    (import.meta.env.VITE_SUBDOMAIN_BASE_DOMAIN as string | undefined) ?? "";
  const fallbackSubdomain =
    (import.meta.env.VITE_CUSTOMER_SUBDOMAIN as string | undefined) ?? "";

  const inferredBaseDomain = (() => {
    if (configuredBaseDomain) {
      return configuredBaseDomain;
    }

    const [hostname, port] = window.location.host.split(":");
    if (!hostname) {
      return window.location.host;
    }

    const labels = hostname.split(".");
    if (labels.length <= 1) {
      return window.location.host;
    }

    const host = labels.slice(1).join(".");
    return port ? `${host}:${port}` : host;
  })();

  return extractSubdomain(window.location.host, inferredBaseDomain) || fallbackSubdomain;
}

export async function getCustomerInstallDetail(installId: string): Promise<TCustomerInstallDetail> {
  const query = new URLSearchParams();
  const subdomain = inferCustomerSubdomain();
  if (subdomain) {
    query.set("subdomain", subdomain);
  }

  const response = await fetch(
    `/bff/portal-api/installs/${installId}/detail${query.toString() ? `?${query.toString()}` : ""}`,
    {
      credentials: "include",
      headers: { Accept: "application/json" },
      cache: "no-store",
    },
  );

  if (!response.ok) {
    throw new Error(`Customer install detail request failed with status ${response.status}`);
  }

  return (await response.json()) as TCustomerInstallDetail;
}
