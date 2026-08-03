import { useEffect, useMemo, useState } from "react";
import { Message } from "@/components/common/Message";
import { useParams, useSearchParams } from "react-router";
import { AppCatalogTable } from "@/components/vendor/apps/AppCatalogTable";
import { PageHeader } from "@/components/vendor/layout/PageHeader";
import {
  forgetDeletedApp,
  getAppCatalog,
  type TAppCatalogItem,
  type TAppCatalogPagination,
  type TAppCatalogStatus,
  updateAppCatalog,
} from "@/lib/api/vendor/get-app-catalog";

const makeSnapshot = (apps: TAppCatalogItem[]) =>
  JSON.stringify({
    order: apps.map((app) => app.id),
    statuses: apps.reduce<Record<string, TAppCatalogStatus>>((acc, app) => {
      acc[app.id] = app.status;
      return acc;
    }, {}),
  });

export const AppsView = () => {
  const { orgId } = useParams<{ orgId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();

  const currentPage = Math.max(Number(searchParams.get("page") || "1") || 1, 1);

  const [apps, setApps] = useState<TAppCatalogItem[]>([]);
  const [pagination, setPagination] = useState<TAppCatalogPagination | null>(
    null,
  );
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [baselineSnapshot, setBaselineSnapshot] = useState<string>("[]");

  useEffect(() => {
    if (!orgId) return;

    const load = async () => {
      setIsLoading(true);
      setLoadError(null);

      try {
        const data = await getAppCatalog(orgId, { page: currentPage });
        const items = data.apps ?? [];
        setApps(items);
        setPagination(data.pagination ?? null);
        setBaselineSnapshot(makeSnapshot(items));
      } catch {
        setLoadError("Unable to load app catalog right now.");
        setApps([]);
        setPagination(null);
      } finally {
        setIsLoading(false);
      }
    };

    void load();
  }, [orgId, currentPage]);

  const currentSnapshot = useMemo(() => makeSnapshot(apps), [apps]);
  const hasChanges = currentSnapshot !== baselineSnapshot;

  const updateQueryPage = (nextPage: number) => {
    const params = new URLSearchParams(searchParams);
    params.set("page", String(nextPage));
    setSearchParams(params, { replace: true });
  };

  const handleMove = (index: number, direction: "up" | "down") => {
    const targetIndex = direction === "up" ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= apps.length) return;

    setApps((prev) => {
      const next = [...prev];
      const [item] = next.splice(index, 1);
      next.splice(targetIndex, 0, item);
      return next;
    });
  };

  const handleStatusChange = (appId: string, status: TAppCatalogStatus) => {
    setApps((prev) =>
      prev.map((app) => (app.id === appId ? { ...app, status } : app)),
    );
  };

  const handleSave = async () => {
    if (!orgId || !hasChanges || isSaving) return;

    setIsSaving(true);
    try {
      const app_ids = apps.map((app) => app.id);
      const app_statuses = apps.reduce<Record<string, TAppCatalogStatus>>(
        (acc, app) => {
          acc[app.id] = app.status;
          return acc;
        },
        {},
      );

      await updateAppCatalog(orgId, { app_ids, app_statuses });
      setBaselineSnapshot(makeSnapshot(apps));
    } catch {
      setLoadError("Unable to save app catalog changes.");
    } finally {
      setIsSaving(false);
    }
  };

  const handleForgetDeletedApp = async (app: TAppCatalogItem) => {
    if (!orgId) return;

    const confirmed = window.confirm(
      "Remove this deleted app from the catalog list? This action cannot be undone.",
    );
    if (!confirmed) return;

    try {
      await forgetDeletedApp(orgId, app.id);
      setApps((prev) => {
        const next = prev.filter((item) => item.id !== app.id);
        setBaselineSnapshot(makeSnapshot(next));
        return next;
      });
    } catch {
      setLoadError("Unable to remove deleted app.");
    }
  };

  return (
    <section className="overflow-auto">
      <PageHeader
        title="App Catalog"
        subtitle="Configure and publish apps for your customer portal."
      />

      {isLoading ? (
        <Message theme="info">Loading apps...</Message>
      ) : loadError ? (
        <Message className="p-6">{loadError}</Message>
      ) : (
        <AppCatalogTable
          orgId={orgId ?? ""}
          apps={apps}
          pagination={pagination}
          hasChanges={hasChanges}
          isSaving={isSaving}
          onMove={handleMove}
          onStatusChange={handleStatusChange}
          onSave={() => void handleSave()}
          onForgetDeletedApp={(app) => void handleForgetDeletedApp(app)}
          onPageChange={updateQueryPage}
        />
      )}
    </section>
  );
};
