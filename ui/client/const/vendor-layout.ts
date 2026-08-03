import type { TIconVariant } from "@/components/common/Icon";

export interface IVendorNavItem {
  kind?: "view-portal" | "view-org";
  suffix?: string;
  href?: string;
  icon: TIconVariant;
  label: string;
  isExternal?: boolean;
  allowWithoutOrg?: boolean;
}

export interface IVendorNavSection {
  label: string;
  requiresOrg?: boolean;
  items: IVendorNavItem[];
}

export const MAIN_NAV_SECTIONS: IVendorNavSection[] = [
  {
    label: "Links",
    requiresOrg: true,
    items: [
      {
        kind: "view-portal",
        icon: "GlobeIcon",
        label: "View Portal",
        isExternal: true,
      },
      {
        kind: "view-org",
        icon: "BuildingsIcon",
        label: "View Org",
        isExternal: true,
      },
    ],
  },
  {
    label: "User Management",
    items: [
      {
        suffix: "accounts",
        icon: "BuildingsIcon",
        label: "Groups",
      },
      {
        suffix: "install-links",
        icon: "LinkIcon",
        label: "Install Links",
      },
      {
        suffix: "installs",
        icon: "PackageIcon",
        label: "Installs",
      },
    ],
  },
  {
    label: "Settings",
    items: [
      {
        suffix: "apps",
        icon: "CubeIcon",
        label: "App Catalog",
      },
      {
        suffix: "portal/branding",
        icon: "PaletteIcon",
        label: "Customer Portal",
      },
      {
        suffix: "team/members",
        icon: "UsersIcon",
        label: "Team",
      },
      {
        suffix: "connection",
        icon: "KeyIcon",
        label: "Org Connection",
      },
    ],
  },
  {
    label: "Resources",
    items: [
      {
        href: "https://docs.nuon.co/guides/customer-portal",
        icon: "BookIcon",
        label: "Documentation",
        isExternal: true,
        allowWithoutOrg: true,
      },
    ],
  },
];

export const VENDOR_BREADCRUMB_LABELS: Record<string, string> = {
  orgs: "Organizations",
  installs: "Installs",
  "install-links": "Install Links",
  apps: "App Catalog",
  portal: "Customer Portal",
  team: "Team",
  connection: "Org Connection",
};
