import { Button } from "@/components/common/Button";
import { Badge } from "@/components/common/Badge";
import { Card } from "@/components/common/Card";
import { CodeBlock } from "@/components/common/CodeBlock";
import { Icon } from "@/components/common/Icon";
import { Loading } from "@/components/common/Loading";
import { Skeleton } from "@/components/common/Skeleton";
import { Text } from "@/components/common/Text";
import type { TCustomerWizardState } from "@/lib/api/customer/install-wizard";
import {
  consoleUrl,
  createStackCmd,
  gcpApplyCmd,
  gcpBackendSnippet,
  updateStackCmd,
} from "../wizard-utils";

export function WorkflowStackStep({
  title,
  headerStatusText,
  isStackComplete,
  nextStepLabel,
  showNextStepButton,
  showViewInstallButton,
  hasStepError,
  isReadOnly,
  isStepComplete,
  overviewPath,
  onNext,
  stackSetup,
  showStackSkeleton,
}: {
  title: string;
  headerStatusText: string;
  isStackComplete: boolean;
  nextStepLabel: string;
  showNextStepButton: boolean;
  showViewInstallButton: boolean;
  hasStepError: boolean;
  isReadOnly: boolean;
  isStepComplete: boolean;
  overviewPath: string;
  onNext: () => void;
  stackSetup: TCustomerWizardState["workflow"]["stack_setup"] | undefined;
  showStackSkeleton: boolean;
}) {
  if (!stackSetup && !showStackSkeleton) {
    return null;
  }

  return (
    <div className="space-y-4">
      <Card className="bg-surface">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex items-center gap-2">
            <Text as="h3" variant="h3" weight="stronger">
              {title}
            </Text>
            <Badge theme={isStackComplete ? "success" : "info"}>
              {!isStackComplete ? <Loading /> : null}
              {headerStatusText}
            </Badge>
          </div>

          <div className="flex flex-wrap gap-2">
            {showNextStepButton ? (
              <Button
                variant="primary"
                size="md"
                disabled={isReadOnly || !isStepComplete}
                onClick={onNext}
              >
                {nextStepLabel}
                <Icon variant="ArrowRightIcon" size={16} weight="bold" />
              </Button>
            ) : null}
            {showViewInstallButton ? (
              <Button
                href={overviewPath}
                variant="primary"
                size="md"
                className={
                  !isStepComplete && !hasStepError
                    ? "opacity-40 pointer-events-none"
                    : undefined
                }
                aria-disabled={!isStepComplete && !hasStepError}
              >
                <Icon variant="CheckCircleIcon" size={16} weight="bold" />
                View install
              </Button>
            ) : null}
          </div>
        </div>
      </Card>

      <Card className="bg-surface">
        {showStackSkeleton ? (
          <div className="space-y-6 py-2">
            <Skeleton lines={1} width="32%" height="22px" />
            <Skeleton lines={1} width="72%" height="28px" />
            <Skeleton lines={1} width="48%" height="28px" />
            <Skeleton lines={1} width="24%" height="58px" />
            <Skeleton lines={1} width="24%" height="28px" />
          </div>
        ) : stackSetup ? (
          <div className="space-y-4">
            {stackSetup.platform === "gcp" ? (
            <div className="space-y-4">
              <div>
                <Text as="div" variant="body" weight="stronger" className="mb-2">
                  1. Clone the install stack module
                </Text>
                <CodeBlock language="bash" showCopy>
                  {
                    "git clone https://github.com/nuonco/install-stacks.git\ncd install-stacks/gcp"
                  }
                </CodeBlock>
              </div>
              <div>
                <Text as="div" variant="body" weight="stronger" className="mb-2">
                  2. Configure remote state
                </Text>
                <CodeBlock language="hcl" showCopy>
                  {gcpBackendSnippet(stackSetup.nuon_install_id || "")}
                </CodeBlock>
              </div>
              {stackSetup.tfvars_content ? (
                <div>
                  <Text
                    as="div"
                    variant="body"
                    weight="stronger"
                    className="mb-2"
                  >
                    3. Save install configuration as install.tfvars
                  </Text>
                  <CodeBlock language="hcl" showCopy>
                    {stackSetup.tfvars_content}
                  </CodeBlock>
                </div>
              ) : null}
              <div>
                <Text as="div" variant="body" weight="stronger" className="mb-2">
                  4. Apply with Terraform
                </Text>
                <CodeBlock language="bash" showCopy>
                  {gcpApplyCmd()}
                </CodeBlock>
              </div>
            </div>
            ) : stackSetup.platform === "azure" ? (
            <div className="space-y-4">
              <div>
                <Text as="div" variant="body" weight="stronger" className="mb-2">
                  1. Login to Azure
                </Text>
                <CodeBlock language="bash" showCopy>
                  {"az login"}
                </CodeBlock>
              </div>
              {stackSetup.nuon_install_id && stackSetup.azure_location ? (
                <div>
                  <Text
                    as="div"
                    variant="body"
                    weight="stronger"
                    className="mb-2"
                  >
                    2. Create resource group
                  </Text>
                  <CodeBlock language="bash" showCopy>
                    {`az group create --name ${stackSetup.nuon_install_id}-rg --location ${stackSetup.azure_location}`}
                  </CodeBlock>
                </div>
              ) : null}
              {stackSetup.azure_template_url ? (
                <div>
                  <Text
                    as="div"
                    variant="body"
                    weight="stronger"
                    className="mb-2"
                  >
                    3. Deploy stack
                  </Text>
                  <CodeBlock language="bash" showCopy>
                    {`az stack group create --name ${stackSetup.nuon_install_id}-stack --resource-group ${stackSetup.nuon_install_id}-rg --template-uri ${stackSetup.azure_template_url} --deny-settings-mode "denyDelete" --aou deleteAll`}
                  </CodeBlock>
                </div>
              ) : null}
            </div>
          ) : (
            <div className="space-y-4">
              {stackSetup.cloudformation_link || stackSetup.template_url ? (
                <div>
                  <Text as="h3" variant="h3" weight="stronger" className="mb-3">
                    Setup your install stack
                  </Text>
                  {stackSetup.cloudformation_link ? (
                    <div className="mb-3">
                      <Button
                        href={stackSetup.cloudformation_link}
                        variant="secondary"
                        size="sm"
                      >
                        Launch CloudFormation
                        <Icon
                          variant="ArrowSquareOutIcon"
                          size={16}
                          weight="bold"
                        />
                      </Button>
                    </div>
                  ) : null}
                  {stackSetup.template_url ? (
                    <div className="space-y-1">
                      <Text as="div" variant="subtext" theme="neutral">
                        CloudFormation template
                      </Text>
                      <CodeBlock language="text" showCopy>
                        {stackSetup.template_url}
                      </CodeBlock>
                    </div>
                  ) : null}
                </div>
              ) : null}

              {stackSetup.cloudformation_link && stackSetup.template_url ? (
                <div className="flex items-center gap-3">
                  <div className="h-px flex-1 bg-border-subtle" />
                  <Text as="span" variant="subtext" theme="neutral">
                    or
                  </Text>
                  <div className="h-px flex-1 bg-border-subtle" />
                </div>
              ) : null}

              {stackSetup.template_url ? (
                <div className="space-y-3">
                  <Text as="h3" variant="h3" weight="stronger">
                    Deploy with AWS CLI
                  </Text>
                  <div className="space-y-1">
                    <Text as="div" variant="subtext" theme="neutral">
                      Create stack
                    </Text>
                    <CodeBlock language="bash" showCopy>
                      {createStackCmd(
                        stackSetup.template_url,
                        stackSetup.stack_name ||
                          `nuon-${stackSetup.nuon_install_id || ""}`,
                        stackSetup.region || "us-east-1",
                      )}
                    </CodeBlock>
                  </div>
                  <div className="space-y-1">
                    <Text as="div" variant="subtext" theme="neutral">
                      Update existing stack
                    </Text>
                    <CodeBlock language="bash" showCopy>
                      {updateStackCmd(
                        stackSetup.template_url,
                        stackSetup.stack_name ||
                          `nuon-${stackSetup.nuon_install_id || ""}`,
                        stackSetup.region || "us-east-1",
                      )}
                    </CodeBlock>
                  </div>
                </div>
              ) : null}

              <div className="space-y-2 mt-6">
                <Text as="h4" variant="body" weight="stronger">
                  Verify your stack
                </Text>
                <Text as="div" variant="subtext" theme="neutral">
                  After running the create or update command above, open the AWS
                  CloudFormation console to monitor your stack progress.
                </Text>
                <Button
                  href={consoleUrl(
                    stackSetup.stack_name ||
                      `nuon-${stackSetup.nuon_install_id || ""}`,
                    stackSetup.region || "us-east-1",
                  )}
                  variant="secondary"
                  size="sm"
                >
                  Open in AWS console
                  <Icon variant="ArrowSquareOutIcon" size={16} weight="bold" />
                </Button>
              </div>
            </div>
            )}
          </div>
        ) : null}
      </Card>
    </div>
  );
}
