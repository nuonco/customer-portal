# Installer App

This is a PoC of a web app, that enables customers to install Nuon apps provided by vendors.

## User Journeys

The app must fulfill the following user stories.

- As a vendor, when I log into the app, an account with the role "vendor" should be created for me.
- As a vendor, after creating an account, I should be able to connect to any instance of a Nuon control plane, by providing an org ID and api token.
- As a vendor, after connecting to a Nuon control plane, I should be able create an "install link" for any app in the connected org.
- As a vendor, I need to be able to create, list, view, and delete install links.

- As a customer, when I click on an install link that a vendor has sent me, an account with the role "customer" should be created for me.
- As a customer, after creating an account, I should be able to create an install of the app the link was made for.
- As a customer, after creating an install, I should be able to manage that install.

## Implementation Status

✅ **Core Infrastructure**
- Go module with all required dependencies
- SQLite database with GORM models (User, NuonOrg, InstallLink, Install)
- Gin web server with role-based routing
- JWT authentication with vendor/customer roles

✅ **Vendor Features**
- Vendor login/signup page
- Organization connection flow (API token + org ID storage)
- Install link creation with SHA-based security
- Install link management (create, view, delete)
- Organization dashboard with link listing

✅ **Customer Features**
- Install link acceptance page
- Customer account creation via install links
- Install creation flow
- Install management dashboard
- Install status tracking

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
   go mod tidy
   VENDOR_PORT=8080 CUSTOMER_PORT=8081 go run main.go
   ```

2. **Access the application:**
   - **Vendor UI**: http://localhost:8080
   - **Customer UI**: http://localhost:8081

3. **Vendor Flow** (port 8080):
   - Enter an email address to create/login as a vendor
   - Connect a Nuon organization (provide org ID and API token)
   - Create install links for your apps
   - Share the install URLs with customers (links point to customer port)

4. **Customer Flow** (port 8081):
   - Click an install link provided by a vendor
   - Enter your email address to create an account
   - Create an installation
   - Manage your installs
   - Note: Customers cannot sign up directly - they must accept an install link first

## Architecture

### Backend
- **Framework**: Gin for HTTP routing and middleware
- **Database**: SQLite with GORM for persistence
- **Authentication**: JWT tokens with role-based access control
- **Authorization**: Custom middleware for vendor/customer separation

### Frontend
- **Rendering**: Server-side HTML templates
- **Styling**: Tailwind CSS for responsive design
- **JavaScript**: Vanilla JS for API interactions
- **State**: JWT tokens stored in localStorage

### Models
- **User**: Email, role (vendor/customer), timestamps
- **NuonOrg**: Connected organizations with API credentials
- **InstallLink**: Shareable links with SHA-based security
- **Install**: Customer installations with status tracking

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VENDOR_PORT` | `8080` | Port for the vendor UI |
| `CUSTOMER_PORT` | `8081` | Port for the customer UI |
| `CUSTOMER_BASE_URL` | `http://localhost:$CUSTOMER_PORT` | Base URL for install links (override for production) |
| `JWT_SECRET` | `your-secret-key` | JWT signing secret (required for production) |

## API Endpoints

### Vendor Server (VENDOR_PORT)
- `GET /` - Redirects to login
- `GET /login` - Vendor login/signup page
- `POST /login` - Vendor authentication (auto-creates accounts)
- `GET /orgs` - Organization management
- `POST /orgs` - Connect new organization
- `GET /orgs/:org_id` - Organization details
- `DELETE /orgs/:org_id` - Delete organization
- `POST /orgs/:org_id/links` - Create install link
- `GET /orgs/:org_id/links/:link_id` - Link details
- `DELETE /orgs/:org_id/links/:link_id` - Delete link

### Customer Server (CUSTOMER_PORT)
- `GET /` - Redirects to login
- `GET /login` - Customer login page (no signup)
- `POST /login` - Customer authentication (existing accounts only)
- `GET /install-link?sha=<sha>` - Install link acceptance page
- `POST /install-link` - Accept link and create account/install
- `GET /installs` - Customer installations
- `GET /installs/:install_id` - Install details
- `PUT /installs/:install_id` - Update install
- `DELETE /installs/:install_id` - Deprovision install
- `POST /installs/:install_id/forget` - Remove install from local DB
- `GET /installs/:install_id/workflows` - Workflow history
- `POST /installs/:install_id/workflows/:workflow_id/approve` - Approve workflow step

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

## Next Steps

1. **Enable Real Nuon API Integration**
   - Uncomment API calls in `pkg/nuon/client.go`
   - Configure proper base URLs for different environments
   - Add proper error handling for API failures

2. **Production Readiness**
   - Add configuration management
   - Implement proper logging
   - Add health checks and metrics
   - Set up database migrations
   - Add comprehensive testing

3. **Enhanced Features**
   - Email notifications for install status
   - Install progress tracking
   - Organization user management
   - Install link expiration
   - Advanced install configuration options
