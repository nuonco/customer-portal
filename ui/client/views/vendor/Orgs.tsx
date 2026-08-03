import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/common/Button";
import { Icon } from "@/components/common/Icon";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { ConnectOrgModal } from "@/components/vendor/org/ConnectOrgModal";
import { getVendorOrgs, type TVendorOrg } from "@/lib/api/vendor/get-orgs";
import { getOrgSession, setOrgSession } from "@/lib/cookies";
import { buildVendorOrgUrl } from "@/utils/vendor-url-utils";

export const OrgsView = () => {
  const [orgs, setOrgs] = useState<TVendorOrg[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [isCreateOrgOpen, setIsCreateOrgOpen] = useState(false);

  useEffect(() => {
    let isMounted = true;

    const loadOrgs = async () => {
      setIsLoading(true);
      setLoadError(null);
      try {
        const results = await getVendorOrgs();
        if (isMounted) {
          setOrgs(results);
        }
      } catch {
        if (isMounted) {
          setLoadError("Unable to load organizations right now.");
          setOrgs([]);
        }
      } finally {
        if (isMounted) {
          setIsLoading(false);
        }
      }
    };

    void loadOrgs();

    return () => {
      isMounted = false;
    };
  }, []);

  const selectedOrg = useMemo(() => {
    const selectedOrgID = getOrgSession();
    if (!selectedOrgID) {
      return orgs[0] ?? null;
    }
    return orgs.find((org) => org.id === selectedOrgID) ?? orgs[0] ?? null;
  }, [orgs]);

  useEffect(() => {
    if (!isLoading && selectedOrg) {
      setOrgSession(selectedOrg.id);
      window.location.assign(buildVendorOrgUrl(selectedOrg.id, "accounts"));
    }
  }, [isLoading, selectedOrg]);

  if (isLoading) {
    return (
      <section className="mx-auto flex max-w-4xl flex-col gap-4 p-6 md:p-8">
        <div className="rounded-xl border border-border-subtle bg-surface p-6 shadow-sm">
          <Text variant="body" theme="neutral">
            Loading organizations...
          </Text>
        </div>
      </section>
    );
  }

  return (
    <section className="mx-auto flex max-w-4xl flex-col gap-4 p-6 md:p-8">
      <div className="rounded-xl border border-border-subtle bg-surface p-6 shadow-sm md:p-8">
        <div className="flex flex-col gap-4 text-center">
          <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-surface-elevated text-text-muted">
            <Icon size={20} variant="BuildingsIcon" />
          </span>

          <Text as="h1" variant="h2" weight="stronger">
            No Organizations Connected
          </Text>

          <Text variant="body" theme="neutral">
            Connect to a Nuon organization to start managing user groups and
            installs.
          </Text>

          {loadError ? <Message theme="warning">{loadError}</Message> : null}

          <div className="pt-2">
            <Button
              onClick={() => setIsCreateOrgOpen(true)}
              variant="primary"
              size="md"
            >
              Connect Your First Org
            </Button>
          </div>
        </div>
      </div>

      <ConnectOrgModal
        isOpen={isCreateOrgOpen}
        onClose={() => setIsCreateOrgOpen(false)}
      />
    </section>
  );
};
