import { Button } from "@/components/common/Button";
import { Card } from "@/components/common/Card";
import { CloudPlatform } from "@/components/common/CloudPlatform";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { useCustomerPortal } from "@/providers/customer-portal-state-provider";
import type { TCloudPlatform } from "@/types";

function StatusBadge({ status }: { status: string }) {
  const isComingSoon = status === "coming_soon";

  return (
    <span
      className={
        isComingSoon
          ? "inline-flex rounded-full bg-orange-100 px-2.5 py-1 text-xs font-medium text-orange-800 dark:bg-orange-950/60 dark:text-orange-300"
          : "inline-flex rounded-full bg-green-100 px-2.5 py-1 text-xs font-medium text-green-800 dark:bg-green-950/60 dark:text-green-300"
      }
    >
      {isComingSoon ? "Coming soon" : "Published"}
    </span>
  );
}

export const CustomerAppsView = () => {
  const { portalState } = useCustomerPortal();

  if (portalState.apps.length === 0) {
    return (
      <Card className="bg-surface">
        <Text as="h2" role="heading" level={2} variant="h3" weight="stronger">
          Apps
        </Text>
        <Text variant="body" theme="neutral">
          No published apps are available yet.
        </Text>
      </Card>
    );
  }

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {portalState.apps.map((app) => (
        <Card key={app.id} className="bg-surface">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <Text
                as="h2"
                role="heading"
                level={2}
                variant="h3"
                weight="stronger"
                className="truncate"
              >
                {app.display_name}
              </Text>
            </div>
            {Boolean(app.platform) && (
              <div className="flex items-center gap-2">
                <CloudPlatform
                  platform={app.platform as TCloudPlatform}
                  displayVariant="icon-only"
                  colorVariant="mono"
                  iconSize="42px"
                  theme="neutral"
                />
              </div>
            )}
          </div>

          {Boolean(app.summary) && (
            <Text variant="body" theme="neutral" className="flex-1">
              {app.summary}
            </Text>
          )}

          <div className="flex flex-wrap gap-2">
            <Button
              href={`/apps/${app.app_id}/install`}
              variant="primary"
              size="md"
            >
              Create install
            </Button>
            <Button href={`/apps/${app.app_id}`} variant="secondary" size="md">
              View details
            </Button>
          </div>
        </Card>
      ))}
    </div>
  );
};

export const CustomerAppDetailView = () => {
  const { currentApp } = useCustomerPortal();

  if (!currentApp) {
    return (
      <Card className="bg-surface">
        <Message theme="warning">
          That app is no longer available in this portal.
        </Message>
        <Button href="/apps" variant="secondary" size="md">
          Back to catalog
        </Button>
      </Card>
    );
  }

  return (
    <Card className="bg-surface">
      <div className="flex items-start justify-between gap-4">
        <div>
          <Text as="h2" role="heading" level={2} variant="h2" weight="stronger">
            {currentApp.display_name}
          </Text>
          <Text as="div" variant="body" theme="neutral">
            {currentApp.app_id}
          </Text>
        </div>
        <StatusBadge status={currentApp.status} />
      </div>

      <Text variant="body" theme="neutral">
        {currentApp.summary ||
          "This app does not have a published overview yet."}
      </Text>

      <p>TODO</p>
    </Card>
  );
};
