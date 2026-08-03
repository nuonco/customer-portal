import { useState } from "react";
import { Button } from "@/components/common/Button";
import { Input } from "@/components/common/form/Input";
import { Message } from "@/components/common/Message";
import { PanelBase, type IPanel } from "@/components/surfaces/Panel";
import { updateVendorProfile } from "@/lib/api/vendor/profile";
import { useVendorAuth } from "@/hooks/use-vendor-auth";
import type { IUser } from "@/types";

interface IProfilePanelProps extends Pick<
  IPanel,
  "isVisible" | "panelId" | "panelKey"
> {
  user: IUser | null;
}

export const VendorProfilePanel = ({
  user,
  isVisible,
  panelId,
  panelKey,
}: IProfilePanelProps) => {
  const { retry } = useVendorAuth();
  const [name, setName] = useState(user?.name ?? "");
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const handleSave = async () => {
    setIsSaving(true);
    setError(null);
    setSaved(false);

    try {
      await updateVendorProfile(name.trim());
      await retry();
      setSaved(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update profile");
    } finally {
      setIsSaving(false);
    }
  };

  const onNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setName(e.target.value);
    setSaved(false);
  };

  return (
    <PanelBase
      heading="Profile Settings"
      size="default"
      isVisible={isVisible}
      panelId={panelId}
      panelKey={panelKey}
    >
      <Input
        id="profile-name"
        type="text"
        value={name}
        onChange={onNameChange}
        labelProps={{
          labelText: "Display Name",
        }}
        helperText="Display name is shown in the topbar."
        helperTextProps={{ theme: "neutral", variant: "subtext" }}
      />

      <Input
        id="profile-email"
        type="email"
        disabled
        value={user?.email ?? ""}
        labelProps={{
          labelText: "Email",
        }}
        helperText="Email address is used for login and notifications."
        helperTextProps={{ theme: "neutral", variant: "subtext" }}
      />

      {error ? <Message theme="warning">{error}</Message> : null}

      {saved ? (
        <Message theme="success">Profile updated successfully.</Message>
      ) : null}

      <div className="mt-2 flex justify-end">
        <Button
          onClick={handleSave}
          disabled={isSaving || !name.trim()}
          variant="primary"
        >
          {isSaving ? "Saving..." : "Save Changes"}
        </Button>
      </div>
    </PanelBase>
  );
};
