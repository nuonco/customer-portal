import { Message } from "@/components/common/Message";
import { CustomerInstallWizard } from "@/components/customer/install-wizard/InstallWizard";
import { useCustomerPortal } from "@/providers/customer-portal-state-provider";

export const CustomerInstallWizardView = () => {
  const { currentApp } = useCustomerPortal();

  if (!currentApp) {
    return (
      <Message theme="warning">
        That app is no longer available in this portal.
      </Message>
    );
  }

  return <CustomerInstallWizard appId={currentApp.app_id} />;
};
