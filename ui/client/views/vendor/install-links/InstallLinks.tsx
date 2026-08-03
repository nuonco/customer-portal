import { useEffect, useMemo, useState } from "react";
import { useParams, useSearchParams } from "react-router";
import { Button } from "@/components/common/Button";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { PageHeader } from "@/components/vendor/layout/PageHeader";
import { CreateInstallLinkModal } from "@/components/vendor/install-links/CreateInstallLinkModal";
import { InstallLinksTable } from "@/components/vendor/install-links/InstallLinksTable";
import {
  getInstallLinks,
  type TInstallLink,
  type TInstallLinkPagination,
} from "@/lib/api/vendor/get-install-links";
import { buildVendorOrgUrl } from "@/utils/vendor-url-utils";

type TTab = "available" | "used";

export const InstallLinksView = () => {
  const { orgId } = useParams<{ orgId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();

  const activeTab = (
    searchParams.get("tab") === "used" ? "used" : "available"
  ) as TTab;
  const currentPage = Math.max(Number(searchParams.get("page") || "1") || 1, 1);

  const [links, setLinks] = useState<TInstallLink[]>([]);
  const [pagination, setPagination] = useState<TInstallLinkPagination | null>(
    null,
  );
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  useEffect(() => {
    if (!orgId) return;

    const load = async () => {
      setIsLoading(true);
      setLoadError(null);
      try {
        const data = await getInstallLinks(orgId, {
          tab: activeTab,
          page: currentPage,
        });
        setLinks(data.links ?? []);
        setPagination(data.pagination);
      } catch {
        setLoadError("Unable to load install links right now.");
        setLinks([]);
        setPagination(null);
      } finally {
        setIsLoading(false);
      }
    };

    void load();
  }, [orgId, activeTab, currentPage]);

  const updateQuery = (tab: TTab, page: number) => {
    const params = new URLSearchParams(searchParams);
    params.set("tab", tab);
    params.set("page", String(page));
    setSearchParams(params, { replace: true });
  };

  const handleTabChange = (tab: TTab) => {
    updateQuery(tab, 1);
  };

  const handlePageChange = (page: number) => {
    updateQuery(activeTab, page);
  };

  const content = useMemo(() => {
    if (isLoading) {
      return (
        <div className="rounded-lg border border-cool-grey-300 dark:border-dark-grey-500 bg-white dark:bg-dark-grey-900 p-12 text-center">
          <Text variant="body" theme="neutral">
            Loading install links...
          </Text>
        </div>
      );
    }

    if (loadError) {
      return (
        <Message className="p-6">
          {loadError}
        </Message>
      );
    }

    return (
      <InstallLinksTable
        orgId={orgId ?? ""}
        links={links}
        pagination={pagination}
        activeTab={activeTab}
        onTabChange={handleTabChange}
        onPageChange={handlePageChange}
        onCreateLink={() => setIsCreateOpen(true)}
      />
    );
  }, [activeTab, isLoading, links, loadError, orgId, pagination]);

  return (
    <section className="stratus-page-content overflow-auto">
      <PageHeader
        title="Install Links"
        subtitle="Create and manage install links for your customers."
        actions={
          <Button variant="primary" onClick={() => setIsCreateOpen(true)}>
            Create Link
          </Button>
        }
      />

      <div className="stratus-page-section">{content}</div>

      {orgId ? (
        <CreateInstallLinkModal
          orgId={orgId}
          isOpen={isCreateOpen}
          onClose={() => setIsCreateOpen(false)}
          onCreated={(linkId) => {
            window.location.assign(buildVendorOrgUrl(orgId, `install-links/${linkId}`));
          }}
        />
      ) : null}
    </section>
  );
};
