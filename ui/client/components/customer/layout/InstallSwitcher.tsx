import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { NavLink } from "react-router";
import { Icon } from "@/components/common/Icon";
import { Text } from "@/components/common/Text";
import { Input } from "@/components/common/form/Input";
import {
  type TCustomerPortalApp,
  type TCustomerPortalInstall,
} from "@/lib/api/customer/get-portal-state";
import { cn } from "@/utils/classnames";

interface IInstallSwitcherProps {
  collapsed: boolean;
  currentInstall: TCustomerPortalInstall | null;
  installs: TCustomerPortalInstall[];
  apps?: TCustomerPortalApp[];
  onRequestExpandNavigation?: () => void;
}

function InstallSwitcherIcon({ install }: { install: TCustomerPortalInstall }) {
  if (install.app_logo_light) {
    if (install.app_logo_dark) {
      return (
        <>
          <img
            src={install.app_logo_light}
            alt=""
            className="h-8 w-8 rounded-md object-cover dark:hidden"
          />
          <img
            src={install.app_logo_dark}
            alt=""
            className="hidden h-8 w-8 rounded-md object-cover dark:block"
          />
        </>
      );
    }

    return (
      <img
        src={install.app_logo_light}
        alt=""
        className="h-8 w-8 rounded-md object-cover"
      />
    );
  }

  return (
    <span className="flex h-7 w-7 items-center justify-center rounded-md bg-surface-muted text-text-muted">
      <Icon variant="PackageIcon" size={14} />
    </span>
  );
}

export const InstallSwitcher = ({
  collapsed,
  currentInstall,
  installs,
  apps = [],
  onRequestExpandNavigation,
}: IInstallSwitcherProps) => {
  const [isOpen, setIsOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [openAfterExpand, setOpenAfterExpand] = useState(false);
  const [isLabelSpaceExpanded, setIsLabelSpaceExpanded] = useState(!collapsed);
  const [showExpandedContent, setShowExpandedContent] = useState(!collapsed);
  const previousCollapsedRef = useRef(collapsed);
  const switcherRef = useRef<HTMLDivElement | null>(null);
  const labelMeasureRef = useRef<HTMLDivElement | null>(null);
  const [labelWidth, setLabelWidth] = useState(0);

  const singleApp = useMemo(
    () => (apps.length === 1 ? apps[0] : null),
    [apps],
  );
  const createInstallHref = singleApp ? `/apps/${singleApp.app_id}/install` : "/apps";
  const createInstallLabel = singleApp ? "Create install" : "Browse apps";

  const visibleInstalls = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) {
      return installs;
    }

    return installs.filter((install) =>
      install.name.toLowerCase().includes(query),
    );
  }, [installs, search]);

  useEffect(() => {
    const onPointerDown = (event: MouseEvent) => {
      if (!switcherRef.current?.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    const onEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
      }
    };

    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onEscape);

    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onEscape);
    };
  }, []);

  useEffect(() => {
    if (!isOpen) {
      setSearch("");
    }
  }, [isOpen]);

  useEffect(() => {
    if (!collapsed && openAfterExpand) {
      const timeout = setTimeout(() => {
        setIsOpen(true);
        setOpenAfterExpand(false);
      }, 100);

      return () => clearTimeout(timeout);
    }
  }, [collapsed, openAfterExpand]);

  useEffect(() => {
    const wasCollapsed = previousCollapsedRef.current;
    previousCollapsedRef.current = collapsed;

    if (collapsed) {
      setShowExpandedContent(false);
      setIsLabelSpaceExpanded(false);
      return;
    }

    if (!wasCollapsed) {
      setIsLabelSpaceExpanded(true);
      setShowExpandedContent(true);
      return;
    }

    setShowExpandedContent(false);
    setIsLabelSpaceExpanded(true);

    const timeout = setTimeout(() => {
      setShowExpandedContent(true);
    }, 180);

    return () => clearTimeout(timeout);
  }, [collapsed]);

  const triggerLabel = currentInstall?.name || "No install selected";

  useLayoutEffect(() => {
    const element = labelMeasureRef.current;
    if (!element) {
      return;
    }

    const measure = () =>
      setLabelWidth(Math.ceil(element.getBoundingClientRect().width));
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
  }, [triggerLabel, currentInstall?.app_name]);

  return (
    <div
      ref={switcherRef}
      className={cn("relative", collapsed ? "flex justify-center" : "")}
    >
      <button
        type="button"
        className={cn(
          "flex items-center h-12 rounded-xl border border-border-subtle bg-surface text-left transition-[width,padding,background-color,color,border-color] duration-200 ease-[cubic-bezier(0.22,1,0.36,1)] hover:bg-black/5 dark:hover:bg-white/8",
          collapsed ? "size-12 justify-center p-0" : "h-10 w-full px-3 py-2",
        )}
        title={collapsed ? triggerLabel : undefined}
        aria-label={triggerLabel}
        onClick={() => {
          if (collapsed) {
            setOpenAfterExpand(true);
            onRequestExpandNavigation?.();
            return;
          }

          setIsOpen((value) => !value);
        }}
      >
        <div
          className={cn("flex min-w-0 items-center", collapsed ? "" : "flex-1")}
        >
          {currentInstall ? (
            <InstallSwitcherIcon install={currentInstall} />
          ) : (
            <span className="flex h-8 w-8 items-center justify-center rounded-md bg-surface-muted text-text-muted">
              <Icon variant="MinusIcon" size={16} />
            </span>
          )}

          <div
            aria-hidden={collapsed}
            className={cn(
              "min-w-0 overflow-hidden transition-[width,opacity,transform,margin-left] duration-200 ease-[cubic-bezier(0.22,1,0.36,1)]",
              showExpandedContent
                ? "translate-x-0 opacity-100"
                : "-translate-x-1 opacity-0",
            )}
            style={{
              width: isLabelSpaceExpanded ? labelWidth : 0,
              marginLeft: isLabelSpaceExpanded ? 8 : 0,
              willChange: "width, margin-left, transform, opacity",
            }}
          >
            <div className="min-w-0 whitespace-nowrap">
              <Text
                as="div"
                variant="body"
                className="truncate whitespace-nowrap"
              >
                {triggerLabel}
              </Text>
              {/* {currentInstall?.app_name ? (
                <Text
                  as="div"
                  variant="label"
                  theme="neutral"
                  className="truncate whitespace-nowrap"
                >
                  {currentInstall.app_name}
                </Text>
              ) : null} */}
            </div>
          </div>

          <div
            ref={labelMeasureRef}
            aria-hidden="true"
            className="pointer-events-none absolute -z-10 opacity-0"
          >
            <div className="inline-block whitespace-nowrap">
              <Text as="div" variant="body" className="whitespace-nowrap">
                {triggerLabel}
              </Text>
              {currentInstall?.app_name ? (
                <Text
                  as="div"
                  variant="subtext"
                  theme="neutral"
                  className="whitespace-nowrap"
                >
                  {currentInstall.app_name}
                </Text>
              ) : null}
            </div>
          </div>
        </div>

        <span
          aria-hidden={collapsed}
          className={cn(
            "shrink-0 overflow-hidden transition-[width,opacity,margin-left] duration-200 ease-[cubic-bezier(0.22,1,0.36,1)]",
            showExpandedContent ? "ml-2 w-4 opacity-100" : "ml-0 w-0 opacity-0",
          )}
        >
          <Icon
            variant="CaretUpDownIcon"
            size={14}
            className="shrink-0 text-text-muted"
          />
        </span>
      </button>

      {isOpen ? (
        <div
          className={cn(
            "absolute z-30 overflow-hidden rounded-xl border border-border-subtle bg-surface shadow-2xl",
            collapsed
              ? "left-full top-0 ml-2 w-70"
              : "left-0 right-0 top-[calc(100%+0.5rem)]",
          )}
        >
          {installs.length > 6 ? (
            <div className="border-b border-border-subtle p-2.5">
              <Input
                id="install-switcher-search"
                size="sm"
                placeholder="Search installs..."
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />
            </div>
          ) : null}

          <div className="max-h-64 overflow-y-auto py-1">
            <NavLink
              to="/installs"
              className={cn(
                "flex items-center gap-2.5 px-3 py-2 text-sm transition-colors hover:bg-black/5 dark:hover:bg-white/8",
                currentInstall === null
                  ? "bg-black/8 text-text-primary dark:bg-white/10"
                  : "text-text-primary",
              )}
              onClick={() => setIsOpen(false)}
            >
              <span className="flex h-7 w-7 items-center justify-center rounded-md bg-surface-muted text-text-muted">
                <Icon variant="MinusIcon" size={14} weight="bold" />
              </span>
              <div className="min-w-0 flex-1">
                <Text as="div" variant="body" className="truncate">
                  No install selected
                </Text>
                <Text
                  as="div"
                  variant="subtext"
                  theme="neutral"
                  className="truncate"
                >
                  Browse all installs
                </Text>
              </div>
            </NavLink>

            {visibleInstalls.length > 0 ? (
              visibleInstalls.map((install) => (
                <NavLink
                  key={install.id}
                  to={`/installs/${install.id}/overview`}
                  className={cn(
                    "flex items-center gap-2.5 px-3 py-2 text-sm transition-colors hover:bg-black/5 dark:hover:bg-white/8",
                    currentInstall?.id === install.id
                      ? "bg-black/8 text-text-primary dark:bg-white/10"
                      : "text-text-primary",
                  )}
                  onClick={() => setIsOpen(false)}
                >
                  <InstallSwitcherIcon install={install} />
                  <div className="min-w-0 flex-1">
                    <Text as="div" variant="body" className="truncate">
                      {install.name}
                    </Text>
                    {install.app_name ? (
                      <Text
                        as="div"
                        variant="subtext"
                        theme="neutral"
                        className="truncate"
                      >
                        {install.app_name}
                      </Text>
                    ) : null}
                  </div>
                </NavLink>
              ))
            ) : (
              <Text
                as="div"
                variant="body"
                theme="neutral"
                className="px-4 py-3 text-center"
              >
                No installs found
              </Text>
            )}
          </div>

          <div className="border-t border-border-subtle p-1">
            <NavLink
              to={createInstallHref}
              className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm text-text-primary transition-colors hover:bg-black/5 dark:hover:bg-white/8"
              onClick={() => setIsOpen(false)}
            >
              <span className="flex h-7 w-7 items-center justify-center rounded-md bg-surface-muted text-text-muted">
                <Icon variant="PlusIcon" size={14} weight="bold" />
              </span>
              <span>{createInstallLabel}</span>
            </NavLink>
          </div>
        </div>
      ) : null}
    </div>
  );
};
