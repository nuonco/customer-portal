import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { Button } from "@/components/common/Button";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { PageHeader } from "@/components/vendor/layout/PageHeader";
import {
  deleteInstallLink,
  getInstallLinkDetail,
  type TInstallLinkDetailResponse,
} from "@/lib/api/vendor/get-install-links";
import { buildVendorOrgUrl } from "@/utils/vendor-url-utils";

const formatDateTime = (value: string) => {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;

  return parsed.toLocaleString(undefined, {
    month: "long",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
};

const statusClassName = (used: boolean) =>
  used
    ? "bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300"
    : "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300";

export const InstallLinkDetailView = () => {
  const navigate = useNavigate();
  const { orgId, linkId } = useParams<{ orgId: string; linkId: string }>();

  const [detail, setDetail] = useState<TInstallLinkDetailResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [copied, setCopied] = useState(false);

  const loadDetail = useCallback(
    async (showLoading: boolean) => {
      if (!orgId || !linkId) {
        setIsLoading(false);
        setLoadError("Missing organization or install link identifier.");
        setDetail(null);
        return;
      }

      if (showLoading) {
        setIsLoading(true);
      }

      try {
        const data = await getInstallLinkDetail(orgId, linkId);
        setDetail(data);
        setLoadError(null);
      } catch {
        setLoadError("Unable to load install link right now.");
        setDetail(null);
      } finally {
        if (showLoading) {
          setIsLoading(false);
        }
      }
    },
    [linkId, orgId],
  );

  useEffect(() => {
    void loadDetail(true);
  }, [loadDetail]);

  useEffect(() => {
    if (!detail || !orgId || !linkId || detail.link.used) return;

    const interval = window.setInterval(() => {
      void loadDetail(false);
    }, 5000);

    return () => {
      window.clearInterval(interval);
    };
  }, [detail, linkId, loadDetail, orgId]);

  const handleCopyLink = async () => {
    if (!detail?.install_url) return;

    try {
      await navigator.clipboard.writeText(detail.install_url);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setActionError("Unable to copy the install link.");
    }
  };

  const handleDelete = async () => {
    if (!orgId || !detail?.link.id || isDeleting) return;

    const confirmed = window.confirm(
      "Delete this install link? This action cannot be undone.",
    );
    if (!confirmed) return;

    setActionError(null);
    setIsDeleting(true);
    try {
      await deleteInstallLink(orgId, detail.link.id);
      navigate(buildVendorOrgUrl(orgId, "install-links"), { replace: true });
    } catch {
      setActionError("Failed to delete install link.");
    } finally {
      setIsDeleting(false);
    }
  };

  const installDisplayName = useMemo(() => {
    if (!detail) return "Install Link";
    return (
      detail.link.install?.name || detail.link.name || detail.link.app_name
    );
  }, [detail]);

  return (
    <section className="stratus-page-content overflow-auto">
      <PageHeader
        title="Install Link Details"
        subtitle="View and manage this install link."
      />

      <div className="stratus-page-section space-y-6">
        {isLoading ? (
          <div className="rounded-lg border border-cool-grey-300 dark:border-dark-grey-500 bg-white dark:bg-dark-grey-900 p-12 text-center">
            <Text variant="body" theme="neutral">
              Loading install link...
            </Text>
          </div>
        ) : null}

        {!isLoading && loadError ? (
          <Message className="p-6">
            <p>{loadError}</p>
            <div className="mt-4">
              <Button variant="secondary" onClick={() => void loadDetail(true)}>
                Retry
              </Button>
            </div>
          </Message>
        ) : null}

        {!isLoading && !loadError && detail ? (
          <>
            <div className="rounded-lg border border-cool-grey-300 bg-white p-6 shadow-sm dark:border-dark-grey-500 dark:bg-dark-grey-900">
              <Text as="h2" variant="base" weight="strong">
                Install Information
              </Text>

              <dl className="mt-4 grid grid-cols-2 gap-4">
                <div>
                  <Text variant="label" theme="neutral" as="dt">
                    Name
                  </Text>
                  <Text variant="body" as="dd">
                    {installDisplayName}
                  </Text>
                </div>

                <div>
                  <Text variant="label" theme="neutral" as="dt">
                    Application
                  </Text>
                  <Text variant="body" as="dd">
                    {detail.link.app_name}
                  </Text>
                </div>

                <div>
                  <Text variant="label" theme="neutral" as="dt">
                    App ID
                  </Text>
                  <Text variant="body" as="dd">
                    {detail.link.app_id}
                  </Text>
                </div>

                <div>
                  <Text variant="label" theme="neutral" as="dt">
                    Created
                  </Text>
                  <Text variant="body" as="dd">
                    {formatDateTime(detail.link.created_at)}
                  </Text>
                </div>

                <div className="col-span-2 flex gap-2 items-center border-t border-cool-grey-200 pt-4 dark:border-dark-grey-600">
                  <Button
                    variant="danger"
                    onClick={handleDelete}
                    disabled={isDeleting}
                  >
                    {isDeleting ? "Deleting..." : "Delete Link"}
                  </Button>
                  <Text variant="body" theme="error">
                    This will permanently remove the install link.
                  </Text>
                </div>
              </dl>
            </div>

            <div className="rounded-lg border border-cool-grey-300 bg-white p-6 shadow-sm dark:border-dark-grey-500 dark:bg-dark-grey-900">
              <Text as="h2" variant="base" weight="strong">
                Link Management
              </Text>

              <div className="grid grid-cols-1 gap-4 mt-4">
                <div>
                  <Text variant="label" theme="neutral">
                    Status
                  </Text>
                  <div className="mt-2">
                    <span
                      className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${statusClassName(detail.link.used)}`}
                    >
                      {detail.link.used ? "Accepted" : "Pending"}
                    </span>
                  </div>
                </div>

                {!detail.link.used ? (
                  <div>
                    <Text variant="label" theme="neutral">
                      Install URL
                    </Text>
                    <Text className="block mt-1" variant="body" as="p">
                      Share this link with your customer to start install
                      creation.
                    </Text>
                    <div className="mt-2 rounded-md bg-cool-grey-100 p-4 dark:bg-dark-grey-800">
                      <p className="break-all font-mono text-sm">
                        {detail.install_url}
                      </p>
                      <div className="mt-3">
                        <Button variant="primary" onClick={handleCopyLink}>
                          {copied ? "Copied" : "Copy Install Link"}
                        </Button>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/30">
                    <Text
                      variant="body"
                      weight="strong"
                      className="text-green-800 dark:text-green-300"
                    >
                      This install link has been accepted by a customer.
                    </Text>
                    {detail.customer_dashboard_install_url ||
                    detail.nuon_dashboard_install_url ? (
                      <div className="mt-3 flex flex-wrap gap-2">
                        {detail.customer_dashboard_install_url ? (
                          <Button
                            href={detail.customer_dashboard_install_url}
                            isAnchorTag
                            target="_blank"
                            rel="noopener noreferrer"
                          >
                            View in Customer Portal
                          </Button>
                        ) : null}
                        {detail.nuon_dashboard_install_url ? (
                          <Button
                            href={detail.nuon_dashboard_install_url}
                            isAnchorTag
                            target="_blank"
                            rel="noopener noreferrer"
                            variant="secondary"
                          >
                            View in Dashboard
                          </Button>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                )}

                {actionError ? (
                  <Message className="rounded p-3">{actionError}</Message>
                ) : null}
              </div>
            </div>
          </>
        ) : null}
      </div>
    </section>
  );
};
