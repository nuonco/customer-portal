import { type FormEvent } from "react";
import { Button } from "@/components/common/Button";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { Input } from "@/components/common/form/Input";
import { ModalBase } from "@/components/surfaces/Modal";
import { useCustomerCreateGroupModal } from "./useCreateGroupModal";

export function CustomerCreateGroupModal() {
  const {
    closeCreateGroupModal,
    createGroupError,
    createGroupName,
    isCreateGroupModalOpen,
    isCreatingGroup,
    setCreateGroupName,
    submitCreateGroup,
  } = useCustomerCreateGroupModal();

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    await submitCreateGroup();
  };

  if (!isCreateGroupModalOpen) {
    return null;
  }

  return (
    <ModalBase
      isVisible={isCreateGroupModalOpen}
      onClose={closeCreateGroupModal}
      heading="Create a new group"
      showFooter={false}
      size="default"
      className="max-w-xl"
    >
      <form className="space-y-4" onSubmit={(event) => void handleSubmit(event)}>
        <Text variant="body" theme="neutral">
          Enter your company name to get started. Team members can be invited
          later.
        </Text>

        <Input
          id="new-group-name"
          type="text"
          required
          autoFocus
          labelProps={{ labelText: "Company Name" }}
          placeholder="e.g. Acme Inc."
          value={createGroupName}
          onChange={(event) => setCreateGroupName(event.target.value)}
        />

        {createGroupError ? <Message theme="warning">{createGroupError}</Message> : null}

        <div className="flex items-center justify-end gap-2 border-t border-border-subtle pt-4">
          <Button type="button" variant="secondary" onClick={closeCreateGroupModal}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={isCreatingGroup}>
            {isCreatingGroup ? "Creating..." : "Create"}
          </Button>
        </div>
      </form>
    </ModalBase>
  );
}
