import { Button } from "@/components/common/Button";
import { Icon } from "@/components/common/Icon";
import { useCustomerPortal } from "@/providers/customer-portal-state-provider";

export const InstallButton = ({ showIcon = false }: { showIcon?: boolean }) => {
  const { portalState } = useCustomerPortal();
  const apps = portalState.apps ?? [];
  const singleApp = apps.length === 1 ? apps[0] : null;

  return (
    <Button
      href={singleApp ? `/apps/${singleApp.app_id}/install` : "/apps"}
      variant="primary"
      size="md"
    >
      {showIcon && <Icon variant="PlusIcon" size={16} />}
      Create install
    </Button>
  );
};
