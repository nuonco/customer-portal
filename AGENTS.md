# Customer Dashboard Service

The customer-dashboard service provides a white-label portal for Nuon vendors and their customers. Vendors configure install links and branding; customers use the portal to manage their installations.

## Architecture

- **Framework**: Go with Gin for HTTP routing
- **Database**: PostgreSQL with GORM
- **Authentication**: JWT tokens with OIDC/SAML support
- **Frontend**: Server-side HTML templates (templ) with HTMX and Tailwind CSS

## Authentication Endpoints

### Vendor Routes (`/admin` prefix)

| Endpoint | Handler | Purpose |
|----------|---------|---------|
| `GET /admin/login/` | `VendorLoginPageTempl` | Vendor login page |
| `POST /admin/login/` | `LocalLogin` | Local password auth (when no IdP configured) |
| `GET /admin/logout` | `VendorLogout` | Logout handler |
| `GET /admin/callback` | `AuthCallback` | OIDC callback (code exchange) |
| `POST /admin/callback` | `AuthCallback` | SAML callback (SAMLResponse) |
| `POST /admin/refresh_token` | JWT RefreshHandler | Refresh JWT token |

### Customer Routes (root level)

| Endpoint | Handler | Purpose |
|----------|---------|---------|
| `GET /login` | `CustomerLoginPageTempl` | Customer login page |
| `GET /auth/login` | `BaseDomainLogin` | Initiates OIDC from base domain |
| `GET /auth/callback` | `BaseDomainCallback` | OIDC callback on base domain |
| `GET /auth/complete` | `CompleteSubdomainAuth` | Sets JWT cookie on subdomain after base domain auth |
| `GET /auth/error` | `AuthErrorPage` | Auth error display page |
| `POST /refresh_token` | JWT RefreshHandler | Refresh JWT token |

### Customer Auth Flow (3-step subdomain-aware)

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

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `CUSTOMER_BASE_URL` | `http://localhost:8080` | Base URL for customer portal |
| `SUBDOMAIN_BASE_DOMAIN` | `localhost:8080` | Base domain for org subdomains |
| `JWT_SECRET` | `your-secret-key` | JWT signing secret |
| `NUON_API_URL` | `https://api.nuon.co` | Nuon API URL |
| `DATABASE_URL` | - | PostgreSQL connection string |

### OIDC Configuration (environment fallback)

| Variable | Description |
|----------|-------------|
| `OIDC_CLIENT_ID` | OIDC client ID |
| `OIDC_CLIENT_SECRET` | OIDC client secret |
| `OIDC_ISSUER_URL` | OIDC issuer URL |
| `OIDC_REDIRECT_URI` | OIDC redirect URI |

## Key Directories

```
internal/
├── auth/           # Authentication providers (OIDC, SAML, local)
├── handlers/       # HTTP handlers
├── middleware/     # JWT, role-based access, subdomain detection
├── models/         # GORM models
└── views/          # templ templates
    ├── customerui/ # Customer-facing UI
    └── vendorui/   # Vendor admin UI
```

## Running Locally

```bash
# From monorepo root
./run-nuonctl.sh services dev --dev customer-dashboard
```

## Testing

```bash
cd services/customer-dashboard
go test ./...
```
