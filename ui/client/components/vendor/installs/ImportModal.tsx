import React, { useEffect, useRef, useState } from "react";
import { Button } from "@/components/common/Button";
import { Input } from "@/components/common/form/Input";
import { Icon } from "@/components/common/Icon";
import { SearchInput } from "@/components/common/SearchInput";
import { ModalBase } from "@/components/surfaces/Modal";
import { Text } from "@/components/common/Text";
import {
  importInstall,
  searchNuonInstalls,
  type TSearchResult,
} from "@/lib/api/vendor/get-installs";

interface IImportModal {
  orgId: string;
  onClose: () => void;
  onImported: () => void;
}

export const ImportModal = ({ orgId, onClose, onImported }: IImportModal) => {
  const [step, setStep] = useState<1 | 2>(1);
  const [query, setQuery] = useState("");
  const [searchResults, setSearchResults] = useState<TSearchResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [selected, setSelected] = useState<TSearchResult | null>(null);
  const [customerEmail, setCustomerEmail] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const searchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (query.length < 3) {
      setSearchResults([]);
      return;
    }
    if (searchTimerRef.current) clearTimeout(searchTimerRef.current);
    searchTimerRef.current = setTimeout(async () => {
      setIsSearching(true);
      try {
        const results = await searchNuonInstalls(orgId, query);
        setSearchResults(results);
      } catch {
        setSearchResults([]);
      } finally {
        setIsSearching(false);
      }
    }, 400);
    return () => {
      if (searchTimerRef.current) clearTimeout(searchTimerRef.current);
    };
  }, [orgId, query]);

  const handleSelect = (result: TSearchResult) => {
    setSelected(result);
    setStep(2);
  };

  const handleBack = () => {
    setStep(1);
    setSelected(null);
    setSubmitError(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selected) return;
    setIsSubmitting(true);
    setSubmitError(null);
    try {
      await importInstall(orgId, {
        nuon_install_id: selected.InstallID,
        app_id: selected.AppID,
        customer_email: customerEmail,
      });
      onImported();
      onClose();
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : "Import failed");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <ModalBase
      isVisible
      onClose={onClose}
      heading="Import Install"
      showFooter={false}
      className="max-w-lg"
    >
        {step === 1 ? (
          <div className="flex flex-col gap-4">
            <Text variant="body" theme="neutral">
              Search for an install in the Nuon API to import it into the
              customer portal. You can search installs by install name.
            </Text>
            <div className="flex flex-col gap-1 w-full">
              <SearchInput
                label="Search installs"
                className="w-full md:min-w-0"
                placeholder="Type an install name..."
                value={query}
                onChange={setQuery}
              />
              <Text variant="subtext" theme="neutral" className="opacity-90">
                Enter at least 3 characters to search.
              </Text>
            </div>
            <div className="max-h-64 overflow-y-auto rounded-lg border border-cool-grey-200 dark:border-dark-grey-600 divide-y divide-cool-grey-100 dark:divide-dark-grey-700 empty:hidden">
              {isSearching ? (
                <div className="px-4 py-3 text-sm text-cool-grey-500 dark:text-cool-grey-400">
                  Searching...
                </div>
              ) : query.length >= 3 && searchResults.length === 0 ? (
                <div className="px-4 py-3 text-sm text-cool-grey-500 dark:text-cool-grey-400">
                  No installs found.
                </div>
              ) : (
                searchResults.map((r) => (
                  <div
                    key={r.InstallID}
                    className="flex items-center justify-between gap-3 px-4 py-3 hover:bg-cool-grey-50 dark:hover:bg-dark-grey-800"
                  >
                    <div>
                      <div className="text-sm font-medium text-cool-grey-900 dark:text-white">
                        {r.InstallName}
                      </div>
                      <div className="text-xs text-cool-grey-500 dark:text-cool-grey-400">
                        {r.AppName}
                        {r.Region ? ` · ${r.Region}` : ""}
                        {r.Status ? ` · ${r.Status}` : ""}
                      </div>
                    </div>
                    <Button
                      size="sm"
                      variant="primary"
                      onClick={() => handleSelect(r)}
                    >
                      Select
                      <Icon variant="ArrowRightIcon" size={12} />
                    </Button>
                  </div>
                ))
              )}
            </div>
          </div>
        ) : (
          <form
            onSubmit={(e) => void handleSubmit(e)}
            className="flex flex-col gap-4"
          >
            <div>
              <h2 className="mb-1 block text-sm font-medium text-text-primary">
                Selected install
              </h2>
              <div className="rounded-lg border border-cool-grey-200 dark:border-dark-grey-600 bg-cool-grey-50 dark:bg-dark-grey-800 px-3 py-2 text-sm text-cool-grey-900 dark:text-white">
                {selected?.InstallName} · {selected?.AppName}
              </div>
            </div>
            {submitError ? (
              <p className="text-sm text-red-600 dark:text-red-400">
                {submitError}
              </p>
            ) : null}
            <Input
              id="import-install-customer-email"
              type="email"
              required
              labelProps={{ labelText: 'Assign to customer' }}
              placeholder="customer@example.com"
              value={customerEmail}
              onChange={(e) => setCustomerEmail(e.target.value)}
              helperText="Enter the customer's email. A new account will be created if this email isn't already in the portal."
            />
            <div className="flex items-center justify-between pt-2">
              <Button type="button" variant="secondary" onClick={handleBack}>
                ← Back
              </Button>
              <Button type="submit" variant="primary" disabled={isSubmitting}>
                {isSubmitting ? "Importing..." : "Import Install"}
              </Button>
            </div>
          </form>
        )}
    </ModalBase>
  );
};
