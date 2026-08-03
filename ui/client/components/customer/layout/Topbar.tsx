import { Icon } from "@/components/common/Icon";
import { Text } from "@/components/common/Text";
import { useCustomerNavigation } from "@/hooks/use-customer-navigation";
import type { TCustomerPortalState } from "@/lib/api/customer/get-portal-state";
import { CustomerUserMenu } from "./UserMenu";

type TTopbarProps = {
  portalState: TCustomerPortalState;
};

export const Topbar = ({ portalState }: TTopbarProps) => {
  const navigation = useCustomerNavigation();

  return (
    <header className="sticky top-0 z-20 border-b border-border-subtle bg-background/85 px-4 py-3 backdrop-blur md:px-6">
      <div className="mx-auto flex w-full max-w-7xl items-center justify-between gap-3">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <button
            type="button"
            aria-label="Open sidebar"
            className="rounded-xl p-2 text-text-muted hover:bg-black/5 hover:text-text-primary dark:hover:bg-white/8 md:hidden"
            onClick={navigation.openMobileSidebar}
          >
            <Icon variant="SidebarSimpleIcon" size={20} />
          </button>

          <button
            type="button"
            aria-label={
              navigation.isSidebarMinimized
                ? "Expand sidebar"
                : "Collapse sidebar"
            }
            className="hidden rounded-xl p-2 text-text-muted hover:bg-black/5 hover:text-text-primary dark:hover:bg-white/8 md:inline-flex"
            onClick={navigation.toggleSidebarMinimized}
          >
            <Icon variant="SidebarSimpleIcon" size={20} />
          </button>
          <div className="min-w-0 flex-1">
            <Text as="h1" variant="h3" weight="stronger" className="truncate">
              {navigation.pageTitle}
            </Text>
            <Text
              as="div"
              variant="subtext"
              theme="neutral"
              className="truncate"
            >
              {navigation.pageSubtitle}
            </Text>
          </div>
        </div>

        <CustomerUserMenu
          activeAccount={portalState.active_account}
          userEmail={portalState.user.email}
        />
      </div>
    </header>
  );
};
