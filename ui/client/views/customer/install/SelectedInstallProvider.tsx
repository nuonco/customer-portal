import { Button } from "@/components/common/Button";
import { Card } from "@/components/common/Card";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { useCustomerPortal } from "@/providers/customer-portal-state-provider";
import {
  getCustomerInstallDetail,
  type TCustomerInstallDetail,
} from "@/lib/api/customer/get-install-detail";
import type { TCustomerPortalInstall } from "@/lib/api/customer/get-portal-state";
import { useQuery } from "@tanstack/react-query";
import { createContext, useContext } from "react";
import { Outlet } from "react-router";

type TSelectedInstallContext = {
  appName: string;
  install: TCustomerInstallDetail["install"];
  installDetail: TCustomerInstallDetail;
  legacyBasePath: string;
};

const SelectedInstallContext = createContext<TSelectedInstallContext | null>(
  null,
);

function normalizeInstallDetail(
  installDetail: TCustomerInstallDetail,
  currentInstall: TCustomerPortalInstall,
): TSelectedInstallContext {
  const install = installDetail.install ?? {
    id: currentInstall.id,
    name: currentInstall.name,
    status: currentInstall.status,
    visibility: currentInstall.visibility,
    created_at: currentInstall.created_at,
    region: "",
    app_id: currentInstall.app_id,
    app_name: currentInstall.app_name,
  };

  return {
    install,
    installDetail: {
      ...installDetail,
      install,
      overview: installDetail.overview ?? {
        stack: { status: "", region: "", account_id: "" },
        sandbox: { status: "", repo: "", branch: "", repo_public: false },
        components: [],
        recent_workflows: [],
        input_fields: [],
      },
      stack: installDetail.stack ?? {
        status: "",
        region: "",
        account_id: "",
        vpc: "",
        outputs: {},
        recent_runs: [],
      },
      sandbox: installDetail.sandbox ?? {
        status: "",
        repo: "",
        directory: "",
        branch: "",
        repo_public: false,
        outputs: {},
        recent_runs: [],
      },
      components: installDetail.components ?? {
        items: [],
        deploys: [],
      },
      roles: installDetail.roles ?? { roles: [] },
      policies: installDetail.policies ?? {
        totals: { pass: 0, warn: 0, deny: 0 },
        items: [],
        reports: [],
      },
      audit: installDetail.audit ?? { workflows: [], action_workflows: [] },
      readme: installDetail.readme ?? { markdown: "" },
      legacy_base_path:
        installDetail.legacy_base_path || currentInstall.legacy_base_path,
    },
    appName:
      installDetail.app.display_name ||
      install.app_name ||
      install.app_id ||
      "Install",
    legacyBasePath:
      installDetail.legacy_base_path || currentInstall.legacy_base_path,
  };
}

export function useSelectedInstall(): TSelectedInstallContext {
  const context = useContext(SelectedInstallContext);
  if (!context) {
    throw new Error(
      "useSelectedInstall must be used within CustomerSelectedInstallProvider",
    );
  }

  return context;
}

export const CustomerSelectedInstallProvider = () => {
  const { currentInstall } = useCustomerPortal();

  if (!currentInstall) {
    return (
      <Card className="bg-surface">
        <Message theme="warning">
          That install is no longer available for your current group.
        </Message>
        <Button href="/installs" variant="secondary" size="md">
          Back to installs
        </Button>
      </Card>
    );
  }

  const { data, isLoading, error } = useQuery({
    queryKey: ["customer", "install-detail", currentInstall.id],
    queryFn: () => getCustomerInstallDetail(currentInstall.id),
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  });

  if (isLoading) {
    return (
      <Card className="bg-surface">
        <Text variant="body" theme="neutral">
          Loading install details...
        </Text>
      </Card>
    );
  }

  if (error || !data) {
    return (
      <div className="space-y-4">
        <Message theme="warning">
          {error instanceof Error
            ? error.message
            : "Unable to load install details right now."}
        </Message>
      </div>
    );
  }

  const selectedInstall = normalizeInstallDetail(data, currentInstall);
  const { installDetail } = selectedInstall;

  return (
    <SelectedInstallContext.Provider value={selectedInstall}>
      <div className="space-y-4">
        {installDetail.api_deleted_error ? (
          <Message theme="alert">
            This install appears to have been deleted from the Nuon API. You are
            viewing cached portal data.
          </Message>
        ) : null}

        {installDetail.nuon_api_error ? (
          <Message theme="warning">{installDetail.nuon_api_error}</Message>
        ) : null}

        <Outlet />
      </div>
    </SelectedInstallContext.Provider>
  );
};
