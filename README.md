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

## Implementation Status

✅ **Core Infrastructure**

- Go module with all required dependencies
- PostgreSQL database with GORM models
  - Supports DATABASE_URL or individual DB_* environment variables
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

✅ **Customer Features**

- Install link acceptance page
- Customer account creation via install links
- Install creation flow
- Install management dashboard
- Install status tracking
- **App Catalog** (`/apps`): When an org has published apps, customers can browse and install them without a link. Redirects to `/installs` if no published apps exist.
- **Published App Install** (`/apps/:app_id/install`): Customers can install a published app by providing a name, region, and any required inputs.

✅ **User Interface**

- Responsive HTML templates with Tailwind CSS
- Interactive JavaScript for API calls
- Modal dialogs for forms
- Error handling and success messages

🚧 **Nuon API Integration**

- Basic client wrapper implemented
- Mock data for development/testing
- Real API calls need to be uncommented and tested

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
- **Install** - Customer installations with status tracking (`InstallLinkID` is nullable; nil for published-app installs)
- **PublishedApp** - Apps published to the customer catalog (org_id + app_id, soft-deletable)

**Configuration Models:**
- **AppInputConfig** - App-specific input field configurations
- **AppHealthCheckConfig** - Health check definitions per app
- **AppTheme** - Custom theming and branding per org
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
├── background/     # Background tasks (health check runner)
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

| Variable                | Default                                | Description                                          |
| ----------------------- | -------------------------------------- | ---------------------------------------------------- |
| `PORT`                  | `8080`                                 | Server port                                          |
| `CUSTOMER_BASE_URL`     | `http://localhost:8080`                | Base URL for install links (override for production) |
| `SUBDOMAIN_BASE_DOMAIN` | `localhost:8080`                       | Base domain for org subdomains                       |
| `JWT_SECRET`            | `your-secret-key`                      | JWT signing secret (required for production)         |
| `NUON_API_URL`          | `https://api.nuon.co`                  | Nuon API URL                                         |
| `AUTH_PROVIDER`         | -                                      | OIDC auth provider type (e.g., google, okta, auth0) - falls back to local auth if not configured |
| `AUTH_OIDC_ISSUER_URL`  | -                                      | Issuer URL for the auth provider                     |
| `AUTH_CLIENT_ID`        | -                                      | Auth provider client ID                              |
| `AUTH_REDIRECT_URI`     | `http://localhost:8080/admin/callback` | Auth callback URL the provider will use              |
| `DATABASE_URL`          | -                                      | PostgreSQL connection string                         |

### OIDC Configuration (environment fallback)

| Variable             | Description        |
| -------------------- | ------------------ |
| `OIDC_CLIENT_ID`     | OIDC client ID     |
| `OIDC_CLIENT_SECRET` | OIDC client secret |
| `OIDC_ISSUER_URL`    | OIDC issuer URL    |
| `OIDC_REDIRECT_URI`  | OIDC redirect URI  |

### Additional Configuration Variables

**Logging:**
| Variable    | Default | Description              |
|-------------|---------|--------------------------|
| `LOG_LEVEL` | `INFO`  | Logging verbosity level (DEBUG, INFO, WARN, ERROR) |

**Database (PostgreSQL):**
| Variable       | Description                            |
|----------------|----------------------------------------|
| `DB_HOST`      | Database host                          |
| `DB_NAME`      | Database name                          |
| `DB_USER`      | Database username                      |
| `DB_PORT`      | Database port                          |
| `DB_SSL_MODE`  | SSL mode (disable, require, verify-ca) |
| `DB_REGION`    | AWS region for RDS                     |
| `DB_USE_IAM`   | Enable AWS RDS IAM authentication      |

**Authentication (Extended):**
| Variable                         | Description                           |
|----------------------------------|---------------------------------------|
| `AUTH_CLIENT_SECRET`             | OIDC client secret (required)         |
| `AUTH_POST_LOGOUT_REDIRECT_URI`  | Post-logout redirect URL              |
| `AUTH_OIDC_SCOPES`               | Custom OIDC scopes (space-separated)  |

**SAML Configuration:**
| Variable                  | Description                    |
|---------------------------|--------------------------------|
| `AUTH_SAML_IDP_METADATA_URL` | SAML IdP metadata URL       |
| `AUTH_SAML_ENTITY_ID`     | SAML service provider entity ID |
| `AUTH_SAML_ACS_URL`       | SAML assertion consumer URL    |
| `AUTH_SAML_CERTIFICATE`   | SAML signing certificate       |
| `AUTH_SAML_PRIVATE_KEY`   | SAML private key               |

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
- `GET /admin/orgs/:org_id/apps/:app_id/health-checks` - App health configuration
- `PUT /admin/orgs/:org_id/apps/:app_id/health-checks` - Update app health configuration
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
- `GET /admin/orgs/:org_id/customers` - List organization customers
- `GET /admin/orgs/:org_id/customers/:customer_id` - Customer details
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
- `GET /apps` - Customer app catalog (redirects to `/installs` if no published apps)
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
- `POST /installs/:install_id/health-checks/run` - Trigger health checks manually
- `POST /installs/:install_id/workflows/:workflow_id/approve-all` - Approve all pending steps
- `POST /installs/:install_id/workflows/:workflow_id/cancel` - Cancel running workflow

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
