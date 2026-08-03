import { useContext } from "react";
import { InstallWizardContext } from "@/providers/install-wizard-provider";

export function useInstallWizard() {
  const context = useContext(InstallWizardContext);
  if (!context) {
    throw new Error(
      "useInstallWizard must be used within InstallWizardProvider",
    );
  }
  return context;
}
