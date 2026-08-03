import { useLayoutEffect, useRef, useState } from "react";
import { NavLink } from "react-router";
import { Icon, type TIconVariant } from "@/components/common/Icon";
import { Text } from "@/components/common/Text";
import { LogoMark } from "@/components/common/Logo/LogoMark";
import {
  INSTALL_TABS,
  useCustomerNavigation,
} from "@/hooks/use-customer-navigation";
import { InstallSwitcher } from "./InstallSwitcher";
import { type TCustomerPortalState } from "@/lib/api/customer/get-portal-state";
import { cn } from "@/utils/classnames";

type TCustomerSidebarLinkProps = {
  collapsed: boolean;
  icon: TIconVariant;
  isActive: boolean;
  label: string;
  to: string;
};

function SidebarLink({
  collapsed,
  icon,
  isActive,
  label,
  to,
}: TCustomerSidebarLinkProps) {
  const labelMeasureRef = useRef<HTMLSpanElement | null>(null);
  const [labelWidth, setLabelWidth] = useState(0);

  useLayoutEffect(() => {
    const element = labelMeasureRef.current;
    if (!element) {
      return;
    }

    const measure = () => setLabelWidth(Math.ceil(element.scrollWidth));
    const frame = requestAnimationFrame(measure);

    const fontsReady =
      typeof document !== "undefined" && "fonts" in document
        ? document.fonts.ready.then(measure)
        : Promise.resolve();

    const onResize = () => measure();
    window.addEventListener("resize", onResize);

    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("resize", onResize);
      void fontsReady;
    };
  }, [label]);

  return (
    <NavLink
      to={to}
      className={cn(
        "relative flex items-center rounded-xl px-3 py-2.5 text-sm transition-[padding,background-color,color] duration-200",
        collapsed ? "h-10 w-10 justify-center px-0 py-0" : "",
        isActive
          ? "bg-black/8 text-text-primary dark:bg-white/10"
          : "text-text-muted hover:bg-black/5 hover:text-text-primary dark:hover:bg-white/8",
      )}
      title={collapsed ? label : undefined}
    >
      <span className="shrink-0">
        <Icon variant={icon} size={18} />
      </span>
      <span
        aria-hidden={collapsed}
        className={cn(
          "transform-gpu overflow-hidden whitespace-nowrap transition-[width,margin-left,opacity,transform] duration-200 ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:transition-none",
          collapsed
            ? "opacity-0 transform-[translate3d(-4px,0,0)]"
            : "opacity-100 transform-[translate3d(0,0,0)]",
        )}
        style={{
          width: collapsed ? 0 : labelWidth,
          marginLeft: collapsed ? 0 : 12,
          willChange: "width, margin-left, transform, opacity",
        }}
      >
        {label}
      </span>
      <span
        ref={labelMeasureRef}
        aria-hidden="true"
        className="pointer-events-none absolute -z-10 whitespace-nowrap opacity-0"
      >
        {label}
      </span>
    </NavLink>
  );
}

function LogoBlock({
  portalState,
  collapsed,
}: {
  portalState: TCustomerPortalState;
  collapsed: boolean;
}) {
  const title =
    portalState.theme.header_title || portalState.org.name || "Customer Portal";

  return (
    <div className={cn("flex items-center gap-3", collapsed ? "gap-0" : "")}>
      <div className="flex h-12 w-12 items-center justify-center overflow-hidden rounded-xl bg-surface-muted text-text-muted">
        {portalState.theme.logo_dark ? (
          <>
            <img
              src={portalState.theme.logo_light}
              alt=""
              className="max-h-8 max-w-8 object-contain dark:hidden"
            />
            <img
              src={portalState.theme.logo_dark}
              alt=""
              className="hidden max-h-8 max-w-8 object-contain dark:block"
            />
          </>
        ) : portalState.theme.logo_light ? (
          <img
            src={portalState.theme.logo_light}
            alt=""
            className="max-h-8 max-w-8 object-contain"
          />
        ) : (
          <LogoMark />
        )}
      </div>

      <div
        aria-hidden={collapsed}
        className={cn(
          "min-w-0 overflow-hidden transition-all duration-200",
          collapsed
            ? "max-h-0 max-w-0 -translate-x-1 opacity-0"
            : "max-h-12 max-w-55 translate-x-0 opacity-100",
        )}
      >
        <Text as="div" variant="body" weight="strong" className="truncate">
          {title}
        </Text>
      </div>
    </div>
  );
}

export const Sidebar = ({
  portalState,
}: {
  portalState: TCustomerPortalState;
}) => {
  const navigation = useCustomerNavigation();
  const currentInstall = navigation.currentInstall;
  const isCollapsed = navigation.isSidebarCollapsed;

  const sidebarClassName = navigation.isSidebarMinimized
    ? "md:w-[88px]"
    : "md:w-[320px]";

  return (
    <aside
      data-mobile-open={navigation.isMobileSidebarOpen ? "true" : "false"}
      className={cn(
        "fixed inset-y-0 left-0 z-40 h-screen w-70 oveflow-x-visible overflow-y-auto border-r border-border-subtle bg-[color-mix(in_srgb,var(--color-background)_92%,white_8%)] p-3 shadow-xl transition-[width,transform] duration-200 sm:w-[320px] md:static md:translate-x-0 md:p-4 md:shadow-none",
        navigation.isMobileSidebarOpen ? "translate-x-0" : "-translate-x-full",
        sidebarClassName,
      )}
    >
      <div className="flex flex-col gap-4">
        <div
          className={cn(
            "flex gap-3",
            isCollapsed
              ? "items-center justify-center gap-2"
              : "items-center justify-between",
          )}
        >
          <LogoBlock portalState={portalState} collapsed={isCollapsed} />
          <button
            type="button"
            aria-label="Close sidebar"
            className="rounded-xl p-2 text-text-muted hover:bg-black/5 hover:text-text-primary dark:hover:bg-white/8 md:hidden"
            onClick={navigation.closeMobileSidebar}
          >
            <Icon variant="XIcon" size={18} weight="bold" />
          </button>
        </div>

        <InstallSwitcher
          collapsed={isCollapsed}
          currentInstall={currentInstall}
          installs={portalState.installs}
          onRequestExpandNavigation={() => {
            if (navigation.isSidebarMinimized) {
              navigation.toggleSidebarMinimized();
            }
          }}
        />

        {currentInstall ? (
          <div
            className={cn(
              "space-y-1 border-b border-border-subtle pb-3",
              isCollapsed ? "flex flex-col items-center" : "",
            )}
          >
            {INSTALL_TABS.map(({ icon, key, label }) => (
              <SidebarLink
                key={key}
                collapsed={isCollapsed}
                icon={icon}
                isActive={navigation.activeInstallTab === key}
                label={label}
                to={`/installs/${currentInstall.id}/${key}`}
              />
            ))}
          </div>
        ) : null}

        <div
          className={cn(
            "space-y-1",
            isCollapsed ? "flex flex-col items-center" : "",
          )}
        >
          <SidebarLink
            collapsed={isCollapsed}
            icon="StackIcon"
            isActive={navigation.isAllInstallsActive}
            label="Installs"
            to="/installs"
          />
          <SidebarLink
            collapsed={isCollapsed}
            icon="SquaresFourIcon"
            isActive={navigation.isAppCatalogActive}
            label="App Catalog"
            to="/apps"
          />
        </div>
      </div>
    </aside>
  );
};
