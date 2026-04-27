# Installer App

The customer-dashboard service provides a white-label portal for Nuon vendors and their customers. Vendors configure install links and branding, customers use the portal to manage their installs.

## Quick Start

1. **Build and run the application:**

   ```bash
   # From monorepo root
   ./run-nuonctl.sh services dev --dev customer-dashboard
   ```

   Note: Requires Docker/Podman for PostgreSQL. Use `./run-nuonctl.sh scripts exec reset-dependencies` to start infrastructure services.

2. **Access the application:**

   - **Vendor UI**: http://localhost:8080/admin/
   - **Customer UI**: http://<org-subdomain>.localhost:8080/

3. **Vendor Flow** (`/admin/*`):

   - Login via configured OIDC provider
   - Connect a Nuon organization (provide org ID and API token)
   - Create install links for your apps
   - Share the install links with customers

4. **Customer Flow** (`/*`):
   - Click an install link provided by a vendor
   - Log in via OIDC provider, or, optionally, the vendor's own OIDC provider configured in the portal settings..
   - Create an installation
   - Manage your installs
   - Note: Customers cannot sign up directly - they must accept an install link first

## Development Guide

Run the service locally for development using `nuonctl dev --dev=customer-dashboard`. The service listens on port :8080 in all modes (local dev, container, stage, prod).

**Hot reload**: In dev mode (`LIVE_RELOAD=true`), the app exposes a `GET /dev/version` endpoint that returns the server's start timestamp. A client-side script (`static/js/dev-reload.js`) polls this endpoint every 1.5 seconds. When the version changes (i.e. the Go app was rebuilt and restarted), the script fetches the current page and uses Idiomorph to morph the DOM in-place — preserving form state, scroll position, and open/closed UI elements. If the server is unreachable during a build, the script checks nuonctl's webview API (`localhost:7777`) for build status — showing a "Rebuilding..." spinner or a red "Build failed" overlay with the compiler error text.

When making changes, there are a few patterns and conventions to follow on both the frontend and the backend.

A high-level design principal is that this app has two sections. The first is the customer portal that customer users log into and interact with. The second is an admin section, that vendors log into so they can configure the customer portal. While we use the same libraries and design patterns across both, the two sections are kept separate. Each has it's own set of Gin routes, gin handlers, Templ templates, and Tailwind styles.

### Backend

The backend is implemented using Gin, the Nuon Golang SDK, and Gorm on top of Postgres.

#### Gin

The customer and vendor sections of the app each have their own collection of gin routes and handlers. The customer routes can only be accessed from an org subdomain. The vendor routes are hosted under `/admin` at the root domain.

The handlers read and write data from the database, and render it into UI templates. Often, the handlers also read and write data from the Nuon API using the SDK. We should avoid copying Nuon API data into the DB, so we don't have to worry about keeping the data in sync over time.

Each section has it's own login page. By default, they both use the same auth provider, but the customer login can be configured to use a separate one.

### Frontend

The frontend is implemented using Templ, Tailwind, and HTMX.

#### Templ

Templ is used to define the UI. We maintain separate sets of UI elements for the vendor views and the customer views. Each set is comprised of:

- layout: the top-level wrapper used on all pages.
- pages: the pages of the app, used by the handlers to render pages.
- partials: sections of a page, which may be re-used across pages.
- components: modular, re-usable UI elements used to assemble partials and pages.

#### Tailwind

Tailwind allows us to define styles directly in the Templ templates using utility classes. We should avoid writing custom CSS, either one-off styles or custom classes, as much as possible.

#### Icons

Customer UI uses the [Phosphor Icons](https://phosphoricons.com/) font (Bold weight), self-hosted at `static/fonts/`. Use `<i class="ph-bold ph-{icon-name}">` with Tailwind text-size classes for sizing (e.g., `text-base` for 16px, `text-xl` for 20px). Browse available icons at phosphoricons.com.

#### HTMX

We should avoid writing custom Javascript for client-side interactions and state management. HTMX provides most of what we need to handle things like updating page content, updating the browser history, and polling for updates.

**Progressive enhancement pattern:**

Every interaction should work in three layers, each building on the last:

1. **Plain HTML**: Standard `<a>` links and `<form method="POST">` elements. All UI state lives in the URL (query params). POST handlers use POST-Redirect-GET. Pages render fully server-side.
2. **hx-boost**: The `<body hx-boost="true">` turns all links and forms into AJAX requests transparently. Redirects are followed, URLs update, full pages swap in. No per-element attributes needed.
3. **Targeted HTMX**: For interactions that should avoid full page reloads, elements get `hx-get`, `hx-target`, `hx-swap`, `hx-push-url`. Handlers check a `partial` query param and return HTML fragments instead of full pages.

**Conventions:**

- **URL-driven state**: All UI state (active tabs, selected items, panel expanded/collapsed, alerts) should be in query params so views are bookmarkable and survive reloads.
- **`partial` query param**: GET handlers return different response types based on this param (e.g. `partial=panel` returns a panel fragment, no param returns the full page).
- **POST-Redirect-GET**: POST action handlers always redirect. `hx-boost` follows the redirect. Alert messages travel as `alert_type` and `alert_msg` query params in the redirect URL.
- **HTMX on the element, not the form**: For progressive enhancement of forms, place `hx-post` on the submit button (not the form), so the form works natively without JS.

### CSS Asset Hashing — Hands Off `static/`

The dev server and build pipeline automatically compile source CSS, generate hashed filenames, and update the manifest. **No manual steps are needed.**

- **Only edit source files**: `src/customer.css` and `src/vendor.css`
- **Never** read, edit, create, copy, rename, delete, or `git add` anything under `static/css/` or `static/manifest.json`
- These files are build artifacts — treating them as source files will break styles and cause merge conflicts
- `internal/assets/assets.go` computes hashes on the fly in dev mode; the build pipeline handles production

## User Experience Design

There are some conventions and guidelines that we follow for UI/UX design.

### Common Conventions

There are some conventions that we use both the admin and the customer UIs.

- For both the admin and the customer UIs, we should use modular components. Avoid one-offs as much as possible.
- Do not use built-in browser alerts. Always use HTML modals. There is a component for this in both the admin and the customer UIs.

### Admin

There are some conventions that are specific to the admin UI.

- We use tailwind for the admin UI components, and should stick to using the tailwind utility classes. Avoid custom CSS.
- The admin UI uses Stratus, our official design system, which is also used by the dashboard-ui in the /nuonco/nuon repo.
- The Stratus design system is defined in Figma, and we should pull from the Figma project for all admin UI styles and components: https://www.figma.com/design/K3IcokRSyYRkH2tsr1KMhV/Stratus-Design-System

### Customer

- The customer UI does not use Stratus. We keep it's design simple and unbranded so it's easy for vendors to brand and customize.

## Architecture

### Backend

- **Framework**: Gin for HTTP routing and middleware
- **Database**: Postgres with GORM for persistence
- **Authentication**: JWT tokens with OIDC/SAML support and role-based access control
- **Authorization**: Custom middleware for vendor/customer separation

### Frontend

- **Rendering**: Server-side Golang templates, defined using Templ
- **Styling**: Tailwind CSS for responsive design, with separate design systems for the vendor and the customer UIs
- **JavaScript**: HTMX for dynamic interactions (form submissions, partial loading, navigation); minimal vanilla JS
- **State**: JWT tokens stored in browser cookies

### Models

**Core Models:**

- **User** - Email, role (vendor/customer), timestamps
- **NuonOrg** - Connected organizations with API credentials. Has optional `APIURL` field to override the global Nuon API URL per-org (e.g., for staging or self-hosted control planes); set at org creation time.
- **InstallLink** - Shareable links with SHA-based security
- **Install** - Customer installations with status tracking (`InstallLinkID` is nullable; nil for published-app installs). Has `Visibility` (account/private) and optional `CustomerAccountID` for user-group-based sharing.
- **PublishedApp** - Tracks all apps in the catalog with their display order (org_id + app_id, soft-deletable). Has `Status` (`"published"`, `"coming_soon"`, or `"unpublished"`; default `"published"`), `LogoLightBase64` and `LogoDarkBase64` for per-app logos. All apps (including unpublished) have records so drag-and-drop sort order is preserved across page reloads. Coming-soon apps appear in the catalog with a badge but cannot be installed. Customer-facing queries filter to published/coming_soon only.
- **CustomerAccount** - User group that groups customers together within a vendor org. Displayed as "User Group" in the UI. Scoped to org via `OrgID`.
- **CustomerAccountMember** - Links a user to a user group with role (owner/member). A user can belong to multiple user groups in the same org and switch between them via a cookie.
- **CustomerAccountInvite** - Email-based invite for joining a user group. When a user with a matching email logs in, they are automatically added as a member.
  **Configuration Models:**

- **AppInputConfig** - App-specific input field configurations
- **AppTheme** - Custom theming and branding per org (colors, logos, favicon, fonts, login page, color scheme lock, custom CSS, header title). Vendors can upload a custom favicon via Branding settings; it is stored as a base64 data URI in `FaviconBase64` and rendered in the customer portal `<head>`. The `ThemeMode` field (`"auto"`, `"light"`, `"dark"`) controls whether the customer portal follows the system preference or is locked to a specific color scheme. The `CustomCSS` field allows vendors to inject arbitrary CSS into the customer portal; it is appended to the portal stylesheet after all theme variables are applied and served via the `/custom/css/:org_id.css` endpoint. The vendor logo (`LogoLightBase64`, `LogoDarkBase64`) is **not** shown in the header nav — it appears as a centered hero block (`h-16 max-w-xs`) at the top of each main customer page (`/installs`, `/apps`, install link, app install). When `LoginTitle` and `LoginSubtitle` are not customized by the vendor, the login page uses org-aware defaults: the title shows the org name and the subtitle shows `"Manage your <OrgName> installs"` (falling back to `"Customer Portal"` / `"Manage your installs."` if the org name is unavailable). The `HeaderTitle` field sets a custom title displayed next to the logo in the customer portal header; when empty, it defaults to the org name. The `HeaderTitleHidden` field (boolean) hides the header title entirely when set to true.
- **CustomerAuthConfig** - Customer-specific OIDC/SAML settings
- **GitHubRepoConfig** - GitHub integration settings
- **AssetOverride** - Custom asset uploads (logos, icons)
- **TemplateOverride** - Custom email/notification templates

**Organization Models:**

- **OrgInvitation** - Pending team member invitations
- **OrgMember** - Organization membership and roles

### Debug Page (Vendor-Only)

The debug page provides vendors with a diagnostic view of workflow data for a given install. It is accessible only to users with the `vendor` role, via a "Debug" link in the admin bar. Routes are registered in `main.go` under the `installOwnership` group at `/debug/*`.

The debug page serves as the reference implementation for the HTMX progressive enhancement conventions documented above. Key architectural patterns:

- **URL-driven state**: Active tabs, selected step, panel expanded state, and alert messages are all query params. The `buildDebugURL` helper and `debugURLParams` struct construct URLs, and `baseParams()` extracts current state from props to propagate through all generated links.
- **Panel**: The panel shell is always in the DOM (closed and empty when no workflow is selected). Workflow row clicks swap the panel wrapper via `hx-get` with `partial=panel` + `outerHTML`. Close swaps in an empty closed panel from the list endpoint.
- **Tabs**: The `TabBar` component renders `<a>` links with optional `hx-get`/`hx-target` for partial swaps. The server conditionally renders only the active tab's content.
- **Actions**: Workflow actions (approve, cancel, retry) use `<form method="POST">` with `hx-boost` handling the submission. Handlers always redirect with alert params. No custom JS.
- **Container queries**: The steps list and step detail card use Tailwind `@container` queries (`@5xl:`) to go side-by-side when the panel is expanded.

### Overview Page

The overview page (`/installs/:install_id/overview`) follows the same HTMX progressive enhancement conventions as the debug page:

- **URL-driven state**: The active tab (`?tab=history` or `?tab=inputs`) and alert messages (`?alert_type=...&alert_msg=...`) are query params. The `buildOverviewURL` helper and `overviewURLParams` struct construct URLs.
- **Partial rendering**: The handler checks `?partial=content` to return just the content area (for tab switches via HTMX) or the full page (for initial load / no-JS fallback).
- **Tabs**: The `TabBar` component renders History and Inputs tabs with `hx-get`/`hx-target`/`hx-push-url` for partial swaps.
- **Actions**: Reprovision and deprovision use server-rendered confirmation modals containing `<form method="POST">`. Handlers redirect with alert query params (POST-Redirect-GET). No custom JS for form submission.
- **Data inline**: All overview data (stack, sandbox, components, workflows, inputs) is fetched in the handler and rendered server-side. No lazy loading or shimmer.

Key files:
- `internal/views/customerui/theme/pages/install_overview.templ` — Page template, props, URL helpers
- `internal/handlers/customer_install_detail_page.go` — Handler with partial switch
- `internal/handlers/customer_reprovision_install.go` — POST-Redirect-GET reprovision
- `internal/handlers/customer_delete_install.go` — POST-Redirect-GET deprovision

### Install Wizard

The install creation flow uses a multi-step wizard that guides customers through provisioning: **Configure** → **Stack** → **Sandbox** → **Components**.

- **Step 1 (Configure)**: The install creation form, rendered inside the wizard via `CreateInstallFormContent`. All visits to `/apps/:app_id/install` route through the wizard handler (defaulting to `step=inputs`). On first visit (no install yet), the step indicator shows all subsequent steps as "upcoming". The submit button is labeled **Get started**.
- **Confirm step (hidden from indicator)**: After **Get started**, the POST handler renders a Confirm step that summarizes the entered install name, region/location, and inputs. Cancel returns to Configure (form values are restored from `FormPersist` localStorage). Confirm re-POSTs with `confirmed=true`, at which point the install is actually created via the Nuon API and the wizard advances to step 2. The Confirm step pushes `?step=confirm` to the URL but is **not** added to `wizard.StepDefs`, so it does not appear in the step indicator — UX-wise it is part of Configure. POST handlers use HTMX `HX-Retarget`/`HX-Reswap`/`HX-Push-Url` to swap the confirm content into `#wizard-step-wrapper` from the original Configure form submission.
- **Steps 2-4 (Stack, Sandbox, Components)**: Each step shows deployment progress for that phase with a status card, expandable step details, and approve/retry actions. HTMX polls every 3 seconds for status updates. A "Next" button (disabled until the phase completes) advances to the next step.
- **Sandbox Apply card**: During the apply phase, the Apply card polls the terraform workspace state and displays resources as they are created in real time. Uses the shared `WizardResourceList` component (also used by the Plan card's resource changes tab).
- **Component cards**: Each component gets its own card with a type badge (Terraform, Helm, Kubernetes, Pulumi) derived from the `WorkflowStepApproval.Type` field. Plan data is parsed with type-specific parsers (`ParseTerraformPlan`, `ParseHelmPlan`, `ParseKubernetesPlan`) and displayed in approval tabs with appropriate badges and resource/diff lists. Helm and Kubernetes plans show kind/name entries with add/change/destroy action badges.

**URL structure**: `/apps/:app_id/install?step=<step>&install_id=<id>&workflow_id=<wf-id>`

The wizard applies to both published-app and install-link flows. Both POST handlers (`CreateInstallFromApp`, `AcceptInstallLink`) redirect to the wizard after install creation.

**Key architectural patterns:**
- **URL-driven state**: The `step`, `install_id`, and `workflow_id` query params drive all wizard state.
- **Partial rendering**: The handler checks `?partial=content` to return just the step content (for HTMX polling) or the full page.
- **Step transitions**: Step indicator links and "Next" buttons use targeted HTMX (`hx-get` with `partial=content`, `hx-target="#wizard-content"`) instead of `hx-boost` full-page swaps. A small inline script detects navigation direction via `data-step-index` attributes and applies CSS slide-fade animations (`wizard-transition-forward` / `wizard-transition-backward`). Polling swaps (every 3s) are excluded from transitions via a `data-wizard-nav` flag.
- **Step groups**: The handler fetches workflow step groups from the API and filters them per wizard step via `wizard.MatchesWizardStep`. Stack matches the group with `Labels["name"] == "provision-install-stack"`, sandbox matches `Labels["name"] == "provision-sandbox"`, and components matches all groups with `Labels["domain"] == "component"` (one per component).

Key files:
- `internal/views/customerui/theme/pages/install_wizard.templ` — Wizard page template, step indicator, deploy cards, type-specific component tabs
- `internal/views/customerui/theme/components/wizard_resource_list.templ` — Shared resource list component (plan + apply cards)
- `internal/views/customerui/theme/partials/workflows/types.go` — Plan parsers (terraform, helm, kubernetes) and types
- `internal/handlers/customer_install_wizard.go` — Wizard handler, step routing, URL builder, per-component plan data
- `internal/views/customerui/theme/pages/create_install.templ` — Configure step form (`CreateInstallFormContent` used by wizard, `CreateInstallPageContent` used by install-link standalone page)
- `internal/views/customerui/theme/partials/wizard/confirm.templ` — Hidden Confirm step (summary card + Cancel/Confirm)
- `internal/handlers/customer_app_install_page.go` — Thin delegate to wizard handler (defaults `step` to `inputs`)
- `internal/handlers/customer_create_install_from_app.go` — POST redirect to wizard
- `internal/handlers/customer_accept_install_link.go` — POST redirect to wizard (install-link flow)

### Authentication Implementation Details

#### Vendor Routes (`/admin` prefix)

| Endpoint                    | Handler                | Purpose                                      |
| --------------------------- | ---------------------- | -------------------------------------------- |
| `GET /admin/`               | Redirect               | Redirects to login page                      |
| `GET /admin/login/`         | `VendorLoginPageTempl` | Vendor login page                            |
| `POST /admin/login/`        | `LocalLogin`           | Local password auth (when no IdP configured) |
| `GET /admin/register`       | `RegisterPageTempl`    | Vendor registration page                     |
| `POST /admin/register`      | `Register`             | Create vendor account                        |
| `GET /admin/logout`         | `VendorLogout`         | Logout handler                               |
| `GET /admin/callback`       | `AuthCallback`         | OIDC callback (code exchange)                |
| `POST /admin/callback`      | `AuthCallback`         | SAML callback (SAMLResponse)                 |
| `POST /admin/refresh_token` | JWT RefreshHandler     | Refresh JWT token                            |

#### Customer Routes (root level)

| Endpoint              | Handler                  | Purpose                                             |
| --------------------- | ------------------------ | --------------------------------------------------- |
| `GET /login`          | `CustomerLoginPageTempl` | Customer login page                                 |
| `GET /auth/login`     | `BaseDomainLogin`        | Initiates OIDC from base domain                     |
| `GET /auth/callback`  | `BaseDomainCallback`     | OIDC callback on base domain                        |
| `GET /auth/complete`  | `CompleteSubdomainAuth`  | Sets JWT cookie on subdomain after base domain auth |
| `GET /auth/error`     | `AuthErrorPage`          | Auth error display page                             |
| `POST /refresh_token` | JWT RefreshHandler       | Refresh JWT token                                   |

#### Customer Auth Flow (3-step subdomain-aware)

The customer authentication uses a 3-step flow to handle subdomain cookie scoping:

```
1. /login → redirects to → /auth/login (on base domain)
2. /auth/login → OIDC provider → /auth/callback (on base domain, sets temp token)
3. /auth/callback → redirects to → /auth/complete (on subdomain, sets final JWT cookie)
```

This flow solves the problem of cookies being scoped to subdomains. By authenticating on the base domain first, then completing on the subdomain, the JWT cookie is properly scoped.

**Key files:**

- `main.go` - Route registration
- `internal/handlers/handler.go` - Handler implementations
- `internal/auth/customer_provider.go` - Customer OIDC provider factory

## Environment Variables

| Variable                | Default                                | Description                                                                                      |
| ----------------------- | -------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `PORT`                  | `8080`                                 | Server port                                                                                      |
| `CUSTOMER_BASE_URL`     | `http://localhost:8080`                | Base URL for install links (override for production)                                             |
| `SUBDOMAIN_BASE_DOMAIN` | `localhost:8080`                       | Base domain for org subdomains                                                                   |
| `JWT_SECRET`            | `your-secret-key`                      | JWT signing secret (required for production)                                                     |
| `NUON_API_URL`          | `https://api.nuon.co`                  | Nuon API URL                                                                                     |
| `AUTH_PROVIDER`         | -                                      | OIDC auth provider type (e.g., google, okta, auth0) - falls back to local auth if not configured |
| `AUTH_OIDC_ISSUER_URL`  | -                                      | Issuer URL for the auth provider                                                                 |
| `AUTH_CLIENT_ID`        | -                                      | Auth provider client ID                                                                          |
| `AUTH_REDIRECT_URI`     | `http://localhost:8080/admin/callback` | Auth callback URL the provider will use                                                          |
| `DATABASE_URL`          | -                                      | PostgreSQL connection string                                                                     |

### OIDC Configuration (environment fallback)

| Variable             | Description        |
| -------------------- | ------------------ |
| `OIDC_CLIENT_ID`     | OIDC client ID     |
| `OIDC_CLIENT_SECRET` | OIDC client secret |
| `OIDC_ISSUER_URL`    | OIDC issuer URL    |
| `OIDC_REDIRECT_URI`  | OIDC redirect URI  |

### Additional Configuration Variables

**Logging:**
| Variable | Default | Description |
|-------------|---------|--------------------------|
| `LOG_LEVEL` | `INFO` | Logging verbosity level (DEBUG, INFO, WARN, ERROR) |

**Database (PostgreSQL):**
| Variable | Description |
|----------------|----------------------------------------|
| `DB_HOST` | Database host |
| `DB_NAME` | Database name |
| `DB_USER` | Database username |
| `DB_PORT` | Database port |
| `DB_SSL_MODE` | SSL mode (disable, require, verify-ca) |
| `DB_REGION` | AWS region for RDS |
| `DB_USE_IAM` | Enable AWS RDS IAM authentication |

**Authentication (Extended):**
| Variable | Description |
|----------------------------------|---------------------------------------|
| `AUTH_CLIENT_SECRET` | OIDC client secret (required) |
| `AUTH_POST_LOGOUT_REDIRECT_URI` | Post-logout redirect URL |
| `AUTH_OIDC_SCOPES` | Custom OIDC scopes (space-separated) |

**SAML Configuration:**
| Variable | Description |
|---------------------------|--------------------------------|
| `AUTH_SAML_IDP_METADATA_URL` | SAML IdP metadata URL |
| `AUTH_SAML_ENTITY_ID` | SAML service provider entity ID |
| `AUTH_SAML_ACS_URL` | SAML assertion consumer URL |
| `AUTH_SAML_CERTIFICATE` | SAML signing certificate |
| `AUTH_SAML_PRIVATE_KEY` | SAML private key |

Note: Either `DATABASE_URL` or the individual `DB_*` variables can be used for database configuration.

## Shared UI Components

### Shimmer (Loading Placeholder)

The `components.ShimmerBar` and `components.ShimmerLines` components (`internal/views/customerui/theme/components/shimmer.templ`) render animated placeholder bars for loading states.

**Single bar** — specify height and width as Tailwind classes:

```go
@components.ShimmerBar("h-4", "w-3/4")
```

**Multiple lines** — renders a stack of alternating-width bars:

```go
@components.ShimmerLines(3)
```

The `.shimmer` CSS class is defined in `src/customer.css` and uses theme CSS variables for colors, supporting both light and dark mode. Use shimmer placeholders inside card shells so the page layout is visible immediately while data loads via HTMX.

### EmptyState

The `components.EmptyState` component (`internal/views/customerui/theme/components/empty_state.templ`) renders a centered empty-state placeholder with an icon, title, and message. Use it whenever a card, tab, or list has no data to display.

```go
@components.EmptyState(components.EmptyStateProps{
    Title:   "No workflows yet",
    Message: "Run history for workflows will appear here.",
    Icon:    "ph-clock",
})
```

- **Icon**: Any [Phosphor Bold](https://phosphoricons.com/) class (e.g. `"ph-stack"`, `"ph-clock"`). Defaults to `"ph-empty"` when omitted.
- **Compact**: Set `Compact: true` for smaller padding (`py-6` instead of `py-12`) — useful inside cards.
- **Class**: Additional CSS classes appended to the wrapper div.

Special icon colors: `ph-check-circle` renders green, `ph-arrows-clockwise` renders blue. All others use the default muted grey.

## Vendor UI Components

### Tooltip (ContextTooltip)

The `components.Tooltip` component (`internal/views/vendorui/components/tooltip.templ`) supports two modes:

**Simple text tooltip** — pass `nil` for items:

```go
@components.Tooltip("Helpful hint", nil, components.TooltipTop) {
    <span>Hover me</span>
}
```

**Rich tooltip panel** — pass a slice of `TooltipItem` for a titled, scrollable item list matching the Stratus ContextTooltip design:

```go
@components.Tooltip("Resources", []components.TooltipItem{
    {ID: "1", Title: "Docs", Subtitle: "View documentation", Href: "/docs", Icon: "ph-bold ph-book"},
    {ID: "2", Title: "Status", Subtitle: "All systems go"},
}, components.TooltipBottom) {
    <button>Info</button>
}
```

Items with `Href` render as links with a caret-right icon. Items without render as plain text. Positions: `TooltipTop`, `TooltipBottom`, `TooltipLeft`, `TooltipRight`.

## Semantic Button Classes

Every button element in the customer UI templates carries two semantic CSS classes: `button` (applied to all buttons) and a variant class `button-<variant>`. These classes carry **no styles of their own** — they exist purely as stable CSS hooks that vendors can target with custom CSS injected via the Custom CSS branding setting.

### Variants

| Class                            | Applied to                                                                        |
| -------------------------------- | --------------------------------------------------------------------------------- |
| `button button-primary`          | Primary call-to-action buttons (`.btn-theme-primary`)                             |
| `button button-outline`          | Outline/cancel buttons (`.btn-theme-outline`)                                     |
| `button button-neutral`          | Neutral/muted action buttons (`.btn-neutral`), inline workflow cancel buttons     |
| `button button-danger`           | Destructive actions — Forget, logout, `.dropdown-item-danger`                     |
| `button button-icon`             | Icon-only panel control buttons (`.panel-header-btn`, `.panel-close-btn`)         |
| `button button-nav`              | Sidebar navigation links (`.customer-sidebar-link`) and tab buttons (`.nav-item`) |
| `button button-dropdown-trigger` | Dropdown trigger buttons (`.dropdown-trigger`)                                    |
| `button button-dropdown-item`    | Dropdown menu item buttons (`.dropdown-item`)                                     |
| `button button-pagination`       | Pagination navigation buttons (`.pagination-btn`, `.pagination-nav-btn`)          |

### Other Semantic CSS Hooks

| Class           | Applied to                                                                 |
| --------------- | -------------------------------------------------------------------------- |
| `.header-title` | Header title text displayed next to the logo in the customer portal header |

### Example

Vendors can target these hooks inside the Custom CSS field on the Branding settings page:

```css
/* Make all primary buttons use a custom brand color */
.button.button-primary {
  background-color: #e63946;
  border-radius: 2px;
}

/* Make danger buttons more prominent */
.button.button-danger {
  font-weight: 700;
  text-decoration: underline;
}

/* Round all icon buttons */
.button.button-icon {
  border-radius: 50%;
}
```

These class names are considered stable and will not be removed or renamed in patch or minor releases.
