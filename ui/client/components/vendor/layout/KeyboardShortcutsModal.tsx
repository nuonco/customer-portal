import { ModalBase } from "@/components/surfaces/Modal";
import type { IModal } from "@/components/surfaces/Modal";
import { Text } from "@/components/common/Text";
import { KEYBOARD_SHORTCUTS } from "@/const/keyboard-shortcuts";

type TVendorKeyboardShortcutsModalProps = Pick<
  IModal,
  "isVisible" | "modalId" | "modalKey"
>;

export const VendorKeyboardShortcutsModal = ({
  isVisible,
  modalId,
  modalKey,
}: TVendorKeyboardShortcutsModalProps) => {
  return (
    <ModalBase
      heading="Keyboard Shortcuts"
      size="sm"
      showFooter={false}
      isVisible={isVisible}
      modalId={modalId}
      modalKey={modalKey}
    >
      <dl className="flex flex-col gap-3">
        {KEYBOARD_SHORTCUTS.map((shortcut) => (
          <div
            key={shortcut.label}
            className="flex items-center justify-between gap-4"
          >
            <Text className="text-text-muted" variant="body" as="dt">
              {shortcut.label}
            </Text>
            <dd className="flex gap-1">
              {shortcut.keys.map((key) => (
                <kbd
                  key={`${shortcut.label}-${key}`}
                  className="rounded border border-border-subtle bg-surface-elevated px-2 py-0.5 text-xs font-medium"
                >
                  {key}
                </kbd>
              ))}
            </dd>
          </div>
        ))}
      </dl>
    </ModalBase>
  );
};
