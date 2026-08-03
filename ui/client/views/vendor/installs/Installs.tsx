import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "react-router";
import { Button } from "@/components/common/Button";
import { Message } from "@/components/common/Message";
import { Input } from "@/components/common/form/Input";
import { Select } from "@/components/common/form/Select";
import { PageHeader } from "@/components/vendor/layout/PageHeader";
import { Text } from "@/components/common/Text";
import { ForgetConfirmModal } from "@/components/vendor/installs/ForgetConfirmModal";
import { ImportModal } from "@/components/vendor/installs/ImportModal";
import { InstallsTable } from "@/components/vendor/installs/InstallsTable";
import { getInstalls, type TAdminInstall } from "@/lib/api/vendor/get-installs";

const PLATFORM_OPTIONS = [
  { value: "", label: "All" },
  { value: "aws", label: "AWS" },
  { value: "azure", label: "Azure" },
];

export const InstallsView = () => {
  const { orgId } = useParams<{ orgId: string }>();

  const [installs, setInstalls] = useState<TAdminInstall[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [filterEmail, setFilterEmail] = useState("");
  const [filterApp, setFilterApp] = useState("");
  const [filterPlatform, setFilterPlatform] = useState("");

  const [isImportOpen, setIsImportOpen] = useState(false);
  const [forgetTarget, setForgetTarget] = useState<TAdminInstall | null>(null);

  const filterTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const hasLoadedInitiallyRef = useRef(false);

  const fetchInstalls = useCallback(
    async (email: string, app: string, platform: string) => {
      if (!orgId) return;
      setIsLoading(true);
      setLoadError(null);
      try {
        const data = await getInstalls(orgId, { email, app, platform });
        setInstalls(data.installs ?? []);
      } catch {
        setLoadError("Unable to load installs right now.");
        setInstalls([]);
      } finally {
        setIsLoading(false);
      }
    },
    [orgId],
  );

  useEffect(() => {
    void fetchInstalls("", "", "");
    hasLoadedInitiallyRef.current = true;
  }, [fetchInstalls]);

  useEffect(() => {
    if (!hasLoadedInitiallyRef.current) {
      return;
    }

    if (filterEmail === "" && filterApp === "" && filterPlatform === "") {
      return;
    }

    if (filterTimerRef.current) clearTimeout(filterTimerRef.current);
    filterTimerRef.current = setTimeout(() => {
      void fetchInstalls(filterEmail, filterApp, filterPlatform);
    }, 400);
    return () => {
      if (filterTimerRef.current) clearTimeout(filterTimerRef.current);
    };
  }, [filterEmail, filterApp, filterPlatform, fetchInstalls]);

  const handleForgotten = (installId: string) => {
    setInstalls((prev) => prev.filter((i) => i.ID !== installId));
  };

  const hasFilters =
    filterEmail !== "" || filterApp !== "" || filterPlatform !== "";

  return (
    <section className="overflow-auto">
      <PageHeader
        title="Installs"
        subtitle="Installs owned by your customers and tracked by the customer portal."
        actions={
          <Button variant="primary" onClick={() => setIsImportOpen(true)}>
            Import Install
          </Button>
        }
      />

      <div className="flex flex-col gap-4">
        {/* Filter bar */}
        <div className="flex gap-4">
          <Input
            id="installs-filter-owner"
            className="flex-1"
            labelProps={{ labelText: 'Owner' }}
            value={filterEmail}
            onChange={(e) => setFilterEmail(e.target.value)}
            placeholder="Search by owner email"
          />
          <Input
            id="installs-filter-app"
            className="flex-1"
            labelProps={{ labelText: 'App' }}
            value={filterApp}
            onChange={(e) => setFilterApp(e.target.value)}
            placeholder="Search by app name"
          />
          <Select
            id="installs-filter-platform"
            className="w-56"
            labelProps={{ labelText: 'Platform' }}
            value={filterPlatform}
            onChange={(e) => setFilterPlatform(e.target.value)}
            options={PLATFORM_OPTIONS}
          />
        </div>

        {/* Content */}
        {isLoading ? (
          <div className="rounded-lg border border-cool-grey-300 dark:border-dark-grey-500 bg-white dark:bg-dark-grey-900 p-12 text-center">
            <Text variant="body" theme="neutral">
              Loading installs...
            </Text>
          </div>
        ) : loadError ? (
          <Message className="p-6">
            {loadError}
          </Message>
        ) : (
          <InstallsTable
            orgId={orgId ?? ""}
            installs={installs}
            hasFilters={hasFilters}
            onForget={setForgetTarget}
            onImport={() => setIsImportOpen(true)}
          />
        )}
      </div>

      {isImportOpen && orgId ? (
        <ImportModal
          orgId={orgId}
          onClose={() => setIsImportOpen(false)}
          onImported={() =>
            void fetchInstalls(filterEmail, filterApp, filterPlatform)
          }
        />
      ) : null}

      {forgetTarget && orgId ? (
        <ForgetConfirmModal
          install={forgetTarget}
          orgId={orgId}
          onClose={() => setForgetTarget(null)}
          onForgotten={handleForgotten}
        />
      ) : null}
    </section>
  );
};
