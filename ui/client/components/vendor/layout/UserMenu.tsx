import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigationType } from "react-router";
import { Avatar } from "@/components/common/Avatar";
import { Icon } from "@/components/common/Icon";
import type { IUser } from "@/types";
import { logoutVendor } from "@/lib/api/vendor/logout";
import { useSurfaces } from "@/hooks/use-surfaces";
import { VendorProfilePanel } from "@/components/vendor/layout/ProfilePanel";
import { VendorSuperuserPanel } from "@/components/vendor/layout/SuperuserPanel";
import { VendorKeyboardShortcutsModal } from "@/components/vendor/layout/KeyboardShortcutsModal";

type TUserMenuItem = {
  label: string;
  icon: "UserIcon" | "TerminalWindowIcon" | "ShieldCheckIcon";
  onClick: () => void;
  dividerAfter?: boolean;
};

interface IUserMenuProps {
  user: IUser | null;
}

export const VendorUserMenu = ({ user }: IUserMenuProps) => {
  const { addModal, addPanel, modals, panels } = useSurfaces();
  const location = useLocation();
  const navigationType = useNavigationType();
  const [menuOpen, setMenuOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const hydratedSearchRef = useRef<string | null>(null);

  const openProfilePanel = () => {
    addPanel(<VendorProfilePanel user={user} />, "profile");
  };

  const openSuperuserPanel = () => {
    addPanel(<VendorSuperuserPanel />, "superuser");
  };

  const openKeyboardShortcuts = () => {
    addModal(<VendorKeyboardShortcutsModal />, "keyboard-shortcuts");
  };

  const USER_MENU_ITEMS: TUserMenuItem[] = [
    {
      label: "Profile",
      icon: "UserIcon",
      onClick: () => {
        openProfilePanel();
      },
    },
    {
      label: "Keyboard shortcuts",
      icon: "TerminalWindowIcon",
      onClick: () => {
        openKeyboardShortcuts();
      },
    },
    {
      label: "Superuser",
      icon: "ShieldCheckIcon",
      onClick: () => {
        openSuperuserPanel();
      },
      dividerAfter: true,
    },
  ];

  useEffect(() => {
    if (navigationType !== "POP") {
      return;
    }

    if (hydratedSearchRef.current === location.search) {
      return;
    }

    const params = new URLSearchParams(location.search);
    const panelParam = params.get("panel");
    const modalParam = params.get("modal");

    if (panelParam === "profile" && !user) {
      return;
    }

    hydratedSearchRef.current = location.search;

    if (panelParam === "profile" && !panels.some((panel) => panel.key === "profile")) {
      openProfilePanel();
    }

    if (panelParam === "superuser" && !panels.some((panel) => panel.key === "superuser")) {
      openSuperuserPanel();
    }

    if (
      modalParam === "keyboard-shortcuts" &&
      !modals.some((modal) => modal.key === "keyboard-shortcuts")
    ) {
      openKeyboardShortcuts();
    }
  }, [location.search, modals, navigationType, panels, user]);

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

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const activeTag = document.activeElement?.tagName;
      if (event.key === "?" && !["INPUT", "TEXTAREA"].includes(activeTag ?? "")) {
        event.preventDefault();
        openKeyboardShortcuts();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, []);

  const userDisplayName = user?.name || user?.email || "User";

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
        <div className="absolute right-0 z-20 mt-2 w-52 rounded-md border border-border-subtle bg-surface-muted p-1 shadow-lg">
          {USER_MENU_ITEMS.map((item) => (
            <div key={item.label}>
              <button
                className="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-text-primary transition-colors hover:bg-surface-elevated"
                onClick={() => {
                  item.onClick();
                  setMenuOpen(false);
                }}
                type="button"
              >
                <span>{item.label}</span>
                <Icon size={16} variant={item.icon} />
              </button>
              {item.dividerAfter ? <hr className="my-1 border-border-subtle" /> : null}
            </div>
          ))}

          <button
            className="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-text-primary transition-colors hover:bg-surface-elevated"
            onClick={logoutVendor}
            type="button"
          >
            <span>Log out</span>
            <Icon size={16} variant="SignOutIcon" />
          </button>
        </div>
      ) : null}
    </div>
  );
};