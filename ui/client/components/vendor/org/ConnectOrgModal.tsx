import { FormEvent, useEffect, useRef, useState } from "react";
import { Button } from "@/components/common/Button";
import { Message } from "@/components/common/Message";
import { Input } from "@/components/common/form/Input";
import { ModalBase } from "@/components/surfaces/Modal";
import { createOrg } from "@/lib/api/vendor/create-org";
import { setOrgSession } from "@/lib/cookies";
import { buildVendorOrgUrl } from "@/utils/vendor-url-utils";
import { getRuntimeConfig } from "@/lib/runtime-config";

interface IConnectOrgModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const ConnectOrgModal = ({ isOpen, onClose }: IConnectOrgModalProps) => {
  // Blank means the server falls back to its own NUON_API_URL, so show that
  // rather than a hardcoded default the server may not actually be using.
  const defaultApiUrl = getRuntimeConfig().nuonApiUrl || "https://api.nuon.co";
  const [connectError, setConnectError] = useState<string | null>(null);
  const [isConnectingOrg, setIsConnectingOrg] = useState(false);
  const [newOrgId, setNewOrgId] = useState("");
  const [newApiToken, setNewApiToken] = useState("");
  const [newApiUrl, setNewApiUrl] = useState("");
  const orgIdInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!isOpen) {
      setConnectError(null);
      return;
    }

    window.setTimeout(() => {
      orgIdInputRef.current?.focus();
    }, 50);
  }, [isOpen]);

  const handleClose = () => {
    setConnectError(null);
    onClose();
  };

  const handleConnectOrg = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setConnectError(null);
    setIsConnectingOrg(true);

    try {
      const created = await createOrg({
        orgId: newOrgId,
        apiToken: newApiToken,
        apiUrl: newApiUrl || undefined,
      });
      setOrgSession(created.id);
      const redirectPath = created.redirect_to?.startsWith("/")
        ? created.redirect_to
        : buildVendorOrgUrl(created.id, "install-links");
      window.location.assign(redirectPath);
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to connect org";
      setConnectError(message);
    } finally {
      setIsConnectingOrg(false);
    }
  };

  if (!isOpen) {
    return null;
  }

  return (
    <ModalBase
      isVisible={isOpen}
      onClose={handleClose}
      heading="Connect Org"
      showFooter={false}
      className="max-w-xl"
    >
        <form className="space-y-4" onSubmit={handleConnectOrg}>
          <Input
            id="connect-org-id"
            ref={orgIdInputRef}
            labelProps={{ labelText: "Nuon Org ID" }}
            placeholder="org_..."
            required
            value={newOrgId}
            onChange={(e) => setNewOrgId(e.target.value)}
          />

          <Input
            id="connect-org-token"
            labelProps={{ labelText: "API Token" }}
            placeholder="nuon_pat_..."
            required
            type="password"
            value={newApiToken}
            onChange={(e) => setNewApiToken(e.target.value)}
            helperText="To get a new token, run: nuon orgs api-token -j"
          />

          <Input
            id="connect-org-api-url"
            labelProps={{ labelText: "API URL" }}
            placeholder={defaultApiUrl}
            value={newApiUrl}
            onChange={(e) => setNewApiUrl(e.target.value)}
            helperText={`Leave blank to use ${defaultApiUrl}.`}
          />

          {connectError ? (
            <Message className="p-2 text-xs" role="alert">
              {connectError}
            </Message>
          ) : null}

          <div className="flex items-center justify-end gap-2 pt-2">
            <Button type="button" variant="secondary" onClick={handleClose}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" disabled={isConnectingOrg}>
              {isConnectingOrg ? "Connecting..." : "Connect"}
            </Button>
          </div>
        </form>
    </ModalBase>
  );
};
