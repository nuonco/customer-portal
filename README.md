# Installer App

The customer-dashboard service provides a white-label portal for Nuon vendors and their customers. Vendors configure install links and branding, customers use the portal to manage their installs.

## User Journeys

The app must fulfill the following user stories.

### Vendor Journeys

General vendor journeys.

- As a vendor, when I log into the app, an account with the role "vendor" should be created for me.
- As a vendor, after creating an account, I should be able to connect to any instance of a Nuon control plane, by providing an org ID and api token.
- As a vendor, after connecting to a Nuon control plane, I should be able create an "install link" for any app in the connected org.
- As a vendor, I need to be able to create, list, view, and delete install links.

- As a vendor,
  - when I view an org in the vendor dashboard at https://app.nuon.co, there will be an item in the main nav titled "Connect Customer Portal".
  - when I click on "Connect Customer Portal" a modal should open, explaining what the Customer Portal is, and asking me to confirm that I want to connect my org to it.
  - when I confirm that I want to connect, I am taken to the customer portal login at https://customers.nuon.co/admin/login.
  - when I log in, my org is automatically connected, and I land on https://customers.nuon.co/orgs/:org_id/portal/branding.
- As a vendor,
  - when I log in for the first time, I am entered into an onboarding flow that walks me through setting up the portal, configuring my app, and creating my first install link.
- As a vendor,
  - I can embed the customer portal into a page in my own website.

### Customer Journeys

- As a customer,
  - when I follow an install link, I am shown the install link page, on the first step of the install creation flow.
  - when I select the region and provide the required inputs, the page transitions to the next step in the flow, showing me a view of the provision workflow.
  - when the provision is complete, I am redirected to the install detail page.
- As a customer,
  - if I have a single install, I am shown that install's detail page on the home page of the portal.
  - if I have multiple installs, I am shown the installs list on the homepage.
- As a customer,
  - I always see "App Catalog", "Installs", and "My Account" navigation links in the header.
  - when I visit the App Catalog and no apps are published, I see a friendly empty state message.
- As a customer,
  - after logging in via OIDC, if I don't have a customer account, one is automatically created with the name "<first-name>'s Account" (or "My Account" if no name is available).
  - I can create additional company accounts from the "Create New Account" option in the user menu dropdown.
  - as an account owner, I can rename my account from the Account Settings page.
  - as an account owner, I can transfer ownership to another member from the Account Settings page.
  - as an account owner, I can invite teammates by email from the Account Settings page.
  - if the invited email matches an existing user, they are added as a member immediately.
  - if the invited email doesn't match an existing user, a pending invite is created and they auto-join when they next log in.
  - I can toggle any install I own between "account" visibility (shared) and "private" (only me).
  - I can belong to multiple accounts in the same org (e.g. via invites to different company accounts).
  - the user menu dropdown shows the active account name instead of my email.
  - if I have multiple accounts, the dropdown lists other accounts I can switch to.
  - switching accounts sets a cookie and reloads the page, scoping installs to the selected account.
- As a vendor,
  - I can view all customer accounts in the Accounts tab under Customer Management.
  - I can click into an account to see its members and associated installs.

## Implementation Status

✅ **Core Infrastructure**

- Go module with all required dependencies
- PostgreSQL database with GORM models
  - Supports DATABASE*URL or individual DB*\* environment variables
  - AWS RDS IAM authentication support
- Gin web server with role-based routing
- JWT authentication with vendor/customer roles
- OIDC, SAML, and local password authentication providers

✅ **Vendor Features**

- Vendor login/signup page
- Organization connection flow (API token + org ID storage)
- Install link creation with SHA-based security
- Install link management (create, view, delete)
- Organization dashboard with link listing
- **Publish App**: Vendors can publish apps to a customer-facing catalog (`POST /admin/orgs/:org_id/apps/:app_id/publish`). Published apps appear on the customer `/apps` page without requiring an install link.
- **App Catalog Ordering**: Vendors can drag and drop rows on the Apps page to control the display order of published apps in the customer catalog. Clicking "Save Order" persists the order via `PUT /admin/orgs/:org_id/apps/order`.
- **Per-App Logo**: Vendors can upload separate light and dark mode logos for each app via `GET/PUT /admin/orgs/:org_id/apps/:app_id/logo`. Logos are stored as base64 data URIs in the `PublishedApp` model. The Apps table shows a read-only preview; the Logo subnav tab provides the upload UI. In the customer portal, the light logo is shown by default and the dark logo when dark mode is active.

✅ **Customer Features**

- Install link acceptance page
- Customer account creation via install links
- Install creation flow
- Install management dashboard
- Install status tracking
- **App Catalog** (`/apps`): Customers can browse and install published apps without a link. Always accessible; shows a friendly empty state when no apps are published yet. When exactly one app is published, it is displayed as a full-width detail card (with tabs for overview, inputs, secrets, sandbox, components, roles, and policies) instead of a single small grid card.
- **Published App Install** (`/apps/:app_id/install`): Customers can install a published app by providing a name, region, and any required inputs.
- **Customer Accounts**: After OIDC login, customers are required to create or join a company account before accessing installs. Accounts group customers from the same company so they can share installs. Account owners can invite teammates by email; if the email matches an existing user they are added immediately, otherwise they auto-join on next login.
- **Install Visibility**: Install owners can toggle visibility between "account" (shared with all account members) and "private" (only visible to the owner).

✅ **User Interface**

- Responsive HTML templates with Tailwind CSS
- Interactive JavaScript for API calls
- Modal dialogs for forms
- Error handling and success messages

🚧 **Nuon API Integration**

- Basic client wrapper implemented
- Mock data for development/testing
- Real API calls need to be uncommented and tested
- Non-customer-facing inputs with default values are automatically merged into install creation requests

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

Run the service locally for development using `nuonctl dev --dev=customer-dashboard`. When run in dev mode, the service will listen on port :7331, and a dev proxy will listen to :8080. This is to support live reloading in the browser while making changes. When run in container mode, and in stage and prod, the service itself will listen to port :8080.

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

We should avoid writing custom Javascript for client-side interactions and state management. HTMX provides most of what we need to handle things like udpating page content, updating the browser history, and polling for updates.

## User Experience Design

There are some conventions and guidelines that we follow for UI/UX design.

### Common Conventions

There are some conventions that we use both the admin and the customer UIs.

- For both the admin and the customer UIs, we should use modular components. Avoid one-offs as much as possible.
- Do not use built-in browser alerts. Always use HTML modals. There is a component for this in both the admin and the customer UIs.

### Admin

There are some conventions that are specific to the admin UI.

- The admin UI uses Stratus, our official design system, which is also used by the dashboard-ui in the /nuonco/nuon repo.
- We use tailwind for the admin UI components, and should stick to using the tailwind utility classes. Avoid custom CSS.

### Customer

- The customer UI does not use Stratus. We keep it's design simple and unbranded so it's easy for vendors to brand and customize.
- While tailwind is still installed in the customer UI, we should avoid using the tailwind utility classes. Instead, use semantic classes, so that styles are easy for vendors to override with custom CSS.

## Architecture

### Backend

- **Framework**: Gin for HTTP routing and middleware
- **Database**: Postgres with GORM for persistence
- **Authentication**: JWT tokens with OIDC/SAML support and role-based access control
- **Authorization**: Custom middleware for vendor/customer separation

### Frontend

- **Rendering**: Server-side Golang templates, defined using Templ
- **Styling**: Tailwind CSS for responsive design, with separate design systems for the vendor and the customer UIs
- **JavaScript**: Vanilla JS for API interactions, HTMX for loading templates from the backend
- **State**: JWT tokens stored in browser cookies

### Models

**Core Models:**

- **User** - Email, role (vendor/customer), timestamps
- **NuonOrg** - Connected organizations with API credentials
- **InstallLink** - Shareable links with SHA-based security
- **Install** - Customer installations with status tracking (`InstallLinkID` is nullable; nil for published-app installs). Has `Visibility` (account/private) and optional `CustomerAccountID` for account-based sharing.
- **PublishedApp** - Apps published to the customer catalog (org_id + app_id, soft-deletable). Has `LogoLightBase64` and `LogoDarkBase64` for per-app logos.
- **CustomerAccount** - Company account that groups customers together within a vendor org. Scoped to org via `OrgID`.
- **CustomerAccountMember** - Links a user to a customer account with role (owner/member). A user can belong to multiple accounts in the same org and switch between them via a cookie.
- **CustomerAccountInvite** - Email-based invite for joining a customer account. When a user with a matching email logs in, they are automatically added as a member.

**Configuration Models:**

- **AppInputConfig** - App-specific input field configurations
- **AppTheme** - Custom theming and branding per org (colors, logos, favicon, fonts, login page, color scheme lock, custom CSS, header title). Vendors can upload a custom favicon via Branding settings; it is stored as a base64 data URI in `FaviconBase64` and rendered in the customer portal `<head>`. The `ThemeMode` field (`"auto"`, `"light"`, `"dark"`) controls whether the customer portal follows the system preference or is locked to a specific color scheme. The `CustomCSS` field allows vendors to inject arbitrary CSS into the customer portal; it is appended to the portal stylesheet after all theme variables are applied and served via the `/custom/css/:org_id.css` endpoint. The vendor logo (`LogoLightBase64`, `LogoDarkBase64`) is **not** shown in the header nav — it appears as a centered hero block (`h-16 max-w-xs`) at the top of each main customer page (`/installs`, `/apps`, install link, app install). When `LoginTitle` and `LoginSubtitle` are not customized by the vendor, the login page uses org-aware defaults: the title shows `"<OrgName> BYOC"` and the subtitle shows `"Manage your <OrgName> BYOC installs"` (falling back to `"Customer Portal"` / `"Manage your installs."` if the org name is unavailable). The `HeaderTitle` field sets a custom title displayed next to the logo in the customer portal header; when empty, it defaults to `"<OrgName> BYOC"`. The `HeaderTitleHidden` field (boolean) hides the header title entirely when set to true.
- **CustomerAuthConfig** - Customer-specific OIDC/SAML settings
- **GitHubRepoConfig** - GitHub integration settings
- **AssetOverride** - Custom asset uploads (logos, icons)
- **TemplateOverride** - Custom email/notification templates

**Organization Models:**

- **OrgInvitation** - Pending team member invitations
- **OrgMember** - Organization membership and roles

### Key Directories

```
internal/
├── assets/         # Asset manifest management (cache-busting)
├── auth/           # Authentication providers (OIDC, SAML, local)
├── background/     # Background tasks
├── config/         # Configuration management
├── github/         # GitHub integration
├── handlers/       # HTTP handlers
├── middleware/     # JWT, role-based access, subdomain detection
├── models/         # GORM models
├── shortid/        # ID generation utilities
├── testutil/       # Testing utilities
└── views/          # templ templates
    ├── customerui/ # Customer-facing UI
    └── vendorui/   # Vendor admin UI
```

### Authentication Implementation Details

#### Vendor Routes (`/admin` prefix)

| Endpoint                    | Handler                | Purpose                                      |
| --------------------------- | ---------------------- | -------------------------------------------- |
| `GET /admin/`               | Redirect               | Redirects to login page                      |
| `GET /admin/login/`         | `VendorLoginPageTempl` | Vendor login page                            |
| `POST /admin/login/`        | `LocalLogin`           | Local password auth (when no IdP configured) |
| `GET /admin/register`       | `RegisterPageTempl`    | Vendor registration page                     |
| `POST /admin/register`      | `Register`             | Create vendor account                        |
| `GET /admin/invite`         | `InvitePageTempl`      | Accept team invitation page                  |
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

## API Endpoints

### Vendor Routes (`/admin/*`)

- `GET /admin/` - Redirects to login
- `GET /admin/login/` - Vendor login page (supports OIDC, SAML, or local auth)
- `GET /admin/callback` - OAuth/OIDC callback handler
- `GET /admin/orgs/` - Redirects to first org
- `POST /admin/orgs/` - Connect new organization
- `GET /admin/orgs/:org_id/apps` - Organization apps
- `GET /admin/orgs/:org_id/apps/:app_id` - Redirects to app input configuration page
- `GET /admin/orgs/:org_id/apps/:app_id/inputs` - App input configuration
- `PUT /admin/orgs/:org_id/apps/:app_id/inputs` - Update app input configuration
- `GET /admin/orgs/:org_id/apps/:app_id/logo` - App logo upload page
- `PUT /admin/orgs/:org_id/apps/:app_id/logo` - Save app light/dark logos
- `POST /admin/orgs/:org_id/apps/:app_id/publish` - Publish app to customer catalog
- `DELETE /admin/orgs/:org_id/apps/:app_id/publish` - Remove app from customer catalog
- `GET /admin/orgs/:org_id/links` - Organization install links
- `POST /admin/orgs/:org_id/links` - Create install link
- `GET /admin/orgs/:org_id/links/:link_id` - View link details
- `DELETE /admin/orgs/:org_id/links/:link_id` - Delete link
- `GET /admin/register` - Vendor registration page
- `POST /admin/register` - Create vendor account
- `GET /admin/invite` - Accept team invitation
- `POST /admin/org/create` - Create new organization
- `POST /admin/org/switch` - Switch active organization context
- `GET /admin/profile/panel` - User profile panel
- `PUT /admin/profile/` - Update user profile
- `GET /admin/orgs/:org_id/installs` - List all customer installs tracked in the portal
- `GET /admin/orgs/:org_id/installs/search-nuon` - Search Nuon API for installs to import (HTMX)
- `POST /admin/orgs/:org_id/installs/import` - Import an existing Nuon install and assign to a customer
- `POST /admin/orgs/:org_id/installs/:install_id/forget` - Remove an install from the portal (vendor-only; does not deprovision infrastructure)
- `GET /admin/orgs/:org_id/customers` - List organization customers
- `GET /admin/orgs/:org_id/customers/:customer_id` - Customer details
- `GET /admin/orgs/:org_id/accounts` - List customer company accounts
- `GET /admin/orgs/:org_id/accounts/:account_id` - Account detail with members and installs
- `GET /admin/orgs/:org_id/team/members` - List team members
- `GET /admin/orgs/:org_id/team/invites` - List pending invitations
- `GET /admin/orgs/:org_id/portal/*` - Portal settings pages (branding, custom domain, etc.)
- `POST /admin/orgs/:org_id/invitations` - Generate organization invitations
- `DELETE /admin/orgs/:org_id/invitations/:id` - Revoke organization invitation
- `DELETE /admin/orgs/:org_id/members/:user_id` - Remove organization member
- `PUT /admin/orgs/:org_id/settings` - Update general settings
- `PUT /admin/orgs/:org_id/settings/login` - Update login settings
- `POST /admin/orgs/:org_id/settings/login/test` - Test login configuration
- `PUT /admin/orgs/:org_id/settings/dns` - Update DNS settings
- `GET /admin/orgs/:org_id/settings/dns/check` - Verify DNS configuration
- `GET /admin/orgs/:org_id/settings/github` - GitHub integration settings
- `POST /admin/orgs/:org_id/settings/github` - Connect GitHub integration
- `POST /admin/orgs/:org_id/settings/github/sync` - Sync GitHub repository
- `DELETE /admin/orgs/:org_id/settings/github` - Remove GitHub integration
- `PUT /admin/orgs/:org_id/settings/github/templates/:page` - Toggle template override
- `DELETE /admin/orgs/:org_id/settings/github/templates/:page` - Delete template override
- `PUT /admin/orgs/:org_id/settings/github/assets/*path` - Toggle asset override
- `POST /admin/orgs/:org_id/settings/github/bulk-toggle` - Bulk enable/disable overrides
- `GET /admin/debug/user-orgs` - Debug: View user organizations (development)

### Customer Routes (`/*`)

- `GET /` - Redirects to login
- `GET /login` - Customer login page (no signup)
- `POST /login` - Customer authentication (existing accounts only)
- `GET /install-link?sha=<sha>` - Install link acceptance page
- `POST /install-link` - Accept link and create account/install
- `GET /installs` - Customer installations list
- `GET /installs/:install_id` - Installs list with specific install panel open
- `PUT /installs/:install_id` - Update install
- `DELETE /installs/:install_id` - Deprovision install
- `POST /installs/:install_id/forget` - Remove install from local DB
- `GET /installs/:install_id/workflows` - Workflow history
- `POST /installs/:install_id/workflows/:workflow_id/approve` - Approve workflow step
- `GET /install-link/:sha/app-config` - Get app configuration for install link
- `GET /apps` - Customer app catalog (shows empty state if no published apps)
- `GET /apps/:app_id` - App detail page (full info, components, roles, policies)
- `GET /apps/:app_id/install` - Install form for a published app
- `POST /apps/:app_id/install` - Create install from a published app
- `GET /apps/:app_id/config` - Get app config for a published app (unauthenticated)
- `GET /custom/css/:org_id` - Serve organization-specific CSS
- `GET /custom/assets/:org_id/*path` - Serve organization-specific assets
- `GET /installs/:install_id/panel` - Install detail panel (HTMX)
- `GET /installs/:install_id/panel/history` - Workflow history panel (HTMX)
- `GET /installs/:install_id/panel/audit` - Audit logs panel (HTMX)
- `GET /installs/:install_id/inputs` - Get current install inputs
- `PUT /installs/:install_id/inputs` - Update install inputs
- `POST /installs/:install_id/workflows/:workflow_id/approve-all` - Approve all pending steps
- `POST /installs/:install_id/workflows/:workflow_id/cancel` - Cancel running workflow
- `GET /account` - Account settings page (name editing, members, invites)
- `PUT /account` - Update account name (owner only)
- `GET /account/setup` - Redirects to /installs (accounts are auto-created during auth)
- `GET /account/new` - Account creation form for creating additional accounts
- `POST /account/create` - Create customer account and owner membership
- `GET /account/members` - Redirects to /account
- `POST /account/invite` - Invite a user by email (adds immediately if user exists, creates pending invite otherwise)
- `POST /account/switch` - Switch active account (sets cookie, redirects to /installs)
- `POST /account/members/:member_id/transfer-ownership` - Transfer account ownership to another member
- `DELETE /account/invite/:invite_id` - Revoke a pending invite

## Semantic Button Classes

Every button element in the customer UI templates carries two semantic CSS classes: `button` (applied to all buttons) and a variant class `button-<variant>`. These classes carry **no styles of their own** — they exist purely as stable CSS hooks that vendors can target with custom CSS injected via the Custom CSS branding setting.

### Variants

| Class                            | Applied to                                                                    |
| -------------------------------- | ----------------------------------------------------------------------------- |
| `button button-primary`          | Primary call-to-action buttons (`.btn-theme-primary`)                         |
| `button button-secondary`        | Secondary action buttons (`.btn-theme-secondary`)                             |
| `button button-outline`          | Outline/cancel buttons (`.btn-theme-outline`)                                 |
| `button button-neutral`          | Neutral/muted action buttons (`.btn-neutral`), inline workflow cancel buttons |
| `button button-danger`           | Destructive actions — Forget, logout, `.dropdown-item-danger`                 |
| `button button-icon`             | Icon-only panel control buttons (`.panel-header-btn`, `.panel-close-btn`)     |
| `button button-nav`              | Navigation and tab buttons (`.nav-item`)                                      |
| `button button-dropdown-trigger` | Dropdown trigger buttons (`.dropdown-trigger`)                                |
| `button button-dropdown-item`    | Dropdown menu item buttons (`.dropdown-item`)                                 |
| `button button-pagination`       | Pagination navigation buttons (`.pagination-btn`, `.pagination-nav-btn`)      |

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

## Security Features

- JWT-based authentication with role separation
- Secure SHA generation for install links
- API token encryption (stored but hidden from JSON)
- Input validation and error handling
- CORS protection and secure headers

## Development Notes

- The Nuon API integration uses mock data for development
- Real API calls are commented out and need to be enabled
- Database is auto-migrated on startup
- Templates include comprehensive error handling
- All forms include client-side validation
