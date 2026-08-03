import type {
  TCustomerWizardApp,
  TCustomerWizardForm,
} from "@/lib/api/customer/install-wizard";
import type { TInstallWizardFormValues } from "../wizard-utils";
import { ConfigureStep } from "./Configure";

export default {
  title: "Customer/Install Wizard/Configure",
};

const app: TCustomerWizardApp = {
  app_id: "app-payments",
  display_name: "Payments",
  summary: "Install payments infrastructure",
  status: "published",
  logo_light: "",
  logo_dark: "",
  platform: "aws",
};

const awsForm: TCustomerWizardForm = {
  install_name: "payments-prod",
  platform: "aws",
  input_groups: [
    {
      name: "database",
      display_name: "Database",
      description: "Connection and backup settings for the database layer.",
      inputs: [
        {
          name: "db_user",
          display_name: "Database user",
          description: "Admin username used by the app.",
          type: "string",
          default: "payments_admin",
          required: true,
          sensitive: false,
          index: 0,
        },
        {
          name: "db_password",
          display_name: "Database password",
          description: "Strong password for the admin user.",
          type: "string",
          default: "",
          required: true,
          sensitive: true,
          index: 1,
        },
        {
          name: "enable_backups",
          display_name: "Enable backups",
          description: "Turn on daily backups for recovery.",
          type: "bool",
          default: "true",
          required: false,
          sensitive: false,
          index: 2,
        },
      ],
    },
  ],
};

const azureForm: TCustomerWizardForm = {
  install_name: "payments-azure",
  platform: "azure",
  input_groups: [
    {
      name: "networking",
      display_name: "Networking",
      description: "Network and ingress configuration.",
      inputs: [
        {
          name: "allowed_ips",
          display_name: "Allowed IP list",
          description: "JSON list of CIDRs that can access the install.",
          type: "json",
          default: "[\"10.0.0.0/8\"]",
          required: true,
          sensitive: false,
          index: 0,
        },
      ],
    },
  ],
};

function renderStory({
  form,
  initialValues,
  error = null,
  isCreating = false,
}: {
  form: TCustomerWizardForm;
  initialValues: TInstallWizardFormValues;
  error?: string | null;
  isCreating?: boolean;
}) {
  return (
    <div className="max-w-5xl">
      <ConfigureStep
        app={{ ...app, platform: form.platform }}
        form={form}
        initialValues={initialValues}
        error={error}
        isCreating={isCreating}
        onConfirm={() => {}}
      />
    </div>
  );
}

export const AwsDefault = () =>
  renderStory({
    form: awsForm,
    initialValues: {
      installName: awsForm.install_name || "",
      region: "us-east-1",
      location: "",
      inputs: {
        db_user: "payments_admin",
        db_password: "",
        enable_backups: "true",
      },
    },
  });

export const AzureDefault = () =>
  renderStory({
    form: azureForm,
    initialValues: {
      installName: azureForm.install_name || "",
      region: "",
      location: "eastus",
      inputs: {
        allowed_ips: "[\"10.0.0.0/8\", \"192.168.0.0/16\"]",
      },
    },
  });

export const CreatingInstall = () =>
  renderStory({
    form: awsForm,
    initialValues: {
      installName: awsForm.install_name || "",
      region: "us-west-2",
      location: "",
      inputs: {
        db_user: "payments_admin",
        db_password: "",
        enable_backups: "true",
      },
    },
    isCreating: true,
  });

export const WithServerError = () =>
  renderStory({
    form: awsForm,
    initialValues: {
      installName: awsForm.install_name || "",
      region: "us-east-2",
      location: "",
      inputs: {
        db_user: "payments_admin",
        db_password: "",
        enable_backups: "false",
      },
    },
    error: "Unable to validate provided credentials. Please try again.",
  });
