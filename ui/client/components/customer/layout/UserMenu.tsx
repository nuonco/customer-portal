import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { Avatar } from "@/components/common/Avatar";
import { Icon } from "@/components/common/Icon";
import { Message } from "@/components/common/Message";
import type { TCustomerPortalAccount } from "@/lib/api/customer/get-portal-state";

type TCustomerUserMenuProps = {
  activeAccount: TCustomerPortalAccount | null;
  userEmail: string;
};

const menuItemClassName =
  "flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-text-primary transition-colors hover:bg-surface-elevated";

export function CustomerUserMenu({
  activeAccount,
  userEmail,
}: TCustomerUserMenuProps) {
  const navigate = useNavigate();
  const [menuOpen, setMenuOpen] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [menuError, setMenuError] = useState<string | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!menuOpen) {
      return;
    }

    const handleOutsideClick = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };

    document.addEventListener("click", handleOutsideClick);

    return () => {
      document.removeEventListener("click", handleOutsideClick);
    };
  }, [menuOpen]);

  const handleLogout = async () => {
    await fetch("/auth-api/logout", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json" },
    });

    window.location.assign("/");
  };

  const userDisplayName = activeAccount?.name || userEmail;

  return (
    <div className="relative" ref={containerRef}>
      <button
        aria-expanded={menuOpen}
        aria-label="Open user menu"
        className="flex items-center gap-2 rounded-md px-2 py-1.5 transition-colors hover:bg-surface-elevated"
        onClick={() => setMenuOpen((prev) => !prev)}
        type="button"
      >
        <Avatar name={userDisplayName} size="sm" />
        <span className="hidden max-w-[180px] truncate text-sm font-medium md:inline">
          {userDisplayName}
        </span>
        <Icon size={14} variant="CaretDownIcon" />
      </button>

      {menuOpen ? (
        <div className="absolute right-0 z-20 mt-2 w-56 rounded-md border border-border-subtle bg-surface-muted p-1 shadow-lg">
          <button
            className={menuItemClassName}
            onClick={() => {
              setMenuOpen(false);
              void navigate("/account");
            }}
            type="button"
          >
            <span>Settings</span>
            <Icon size={16} variant="UserIcon" />
          </button>

          <button
            className={menuItemClassName}
            onClick={() => void handleLogout()}
            type="button"
          >
            <span>Log out</span>
            <Icon size={16} variant="SignOutIcon" />
          </button>

          {menuError ? (
            <div className="p-1 pt-2">
              <Message theme="warning">{menuError}</Message>
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
