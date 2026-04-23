# Install from App Catalog Flow

User flow for creating an install from the published app catalog.

## Flow Diagram

```
┌─────────────────────┐
│   GET /apps         │
│   App Catalog       │
│   (browse apps)     │
├─────────────────────┤
│ • Grid of published │
│   apps with logos   │
│ • "View" per app    │
└────────┬────────────┘
         │ click app
         ▼
┌─────────────────────┐
│ GET /apps/:app_id   │
│ App Detail          │
│ (optional)          │
├─────────────────────┤
│ • Overview/markdown │
│ • Components, Roles │
│ • "Install" button  │
└────────┬────────────┘
         │ click Install
         ▼
┌─────────────────────┐   not logged in   ┌──────────────┐
│ GET /apps/:id/      │──────────────────▶│ OIDC Login   │
│     install         │                   │ (sign up /   │
│ Step 1: Configure   │◀──────────────────│  log in)     │
├─────────────────────┤   redirects back  └──────────────┘
│ • Install name      │
│ • Region selector   │
│ • Input fields      │
│   (loaded via HTMX) │
│ • "Deploy stack"    │
└────────┬────────────┘
         │ POST /apps/:id/install
         ▼
┌─────────────────────┐
│ ?step=stack         │
│ Step 2: Stack       │
├─────────────────────┤
│ • Infrastructure    │
│   provisioning      │
│ • Step progress     │
│ • Approve/retry     │
│ • Polls every 3s    │
│ • "Next" when done  │
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│ ?step=sandbox       │
│ Step 3: Sandbox     │
├─────────────────────┤
│ • Sandbox deploy    │
│ • Terraform plan    │
│ • Policy reports    │
│ • "Next" when done  │
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│ ?step=components    │
│ Step 4: Components  │
├─────────────────────┤
│ • Component deploy  │
│ • Progress + status │
│ • "View Overview"   │
│   when complete     │
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│ GET /installs/:id/  │
│     overview        │
│ Install Overview    │
├─────────────────────┤
│ • History tab       │
│ • Inputs tab        │
│ • Reprovision/      │
│   deprovision       │
└─────────────────────┘
```

## Route Summary

| Step | Route | Handler | Auth Required |
|------|-------|---------|---------------|
| Browse Apps | `GET /apps` | `CustomerAppsPage` | Yes |
| App Details | `GET /apps/:app_id` | `CustomerAppDetailPage` | Yes |
| Configure | `GET /apps/:app_id/install` | `CustomerAppInstallPage` | OIDC prompt if not |
| Form Fields | `GET /install-form-fields?app_id=:app_id` | `GetInstallFormFields` | No |
| Create | `POST /apps/:app_id/install` | `CreateInstallFromApp` | Yes |
| Wizard (Stack/Sandbox/Components) | `GET /apps/:app_id/install?step=<step>` | `InstallWizardPage` | Yes (install owner) |
| Overview | `GET /installs/:id/overview` | Install overview handler | Yes (install owner) |

## Notes

- All wizard steps share the same route (`GET /apps/:app_id/install`) with `step`, `install_id`, and `workflow_id` query params driving state.
- Steps 2-4 poll via HTMX every 3 seconds for live progress updates.
- The App Detail page is optional -- users can also click "Install" directly from the catalog grid if only one app is published.
- The same wizard flow applies to install-link installs, but initiated via `POST /install-link/` instead.

## Page Wireframes

### 1. App Catalog (multi-app)

```
         1111111111222222222233333333334444444444555555555566666
1234567890123456789012345678901234567890123456789012345678901234
+--------------------------------------------------------------+
| [squares-four]  App Catalog                                  |
+--------------------------------------------------------------+
|                                                              |
| +----------------+ +----------------+ +----------------+     |
| | [logo]    [AWS]| | [logo]  [Azure]| | [logo]    [GCP]|     |
| |                | |                | |                |     |
| | My App         | | Another App    | | Third App      |     |
| |                | |                | |                |     |
| | Description    | | Description    | | Description    |     |
| | text here...   | | text here...   | | text here...   |     |
| |                | |                | |                |     |
| |        View -> | |        View -> | |        View -> |     |
| +----------------+ +----------------+ +----------------+     |
|                                                              |
+--------------------------------------------------------------+
```

### 1b. App Catalog (single app -- shows full detail inline)

```
         1111111111222222222233333333334444444444555555555566666
1234567890123456789012345678901234567890123456789012345678901234
+--------------------------------------------------------------+
| [squares-four]  App Catalog                 [ Install ]      |
+--------------------------------------------------------------+
|                                                              |
| +----------------------------------------------------------+ |
| | [logo]  My Application                         [AWS]     | |
| |                                                          | |
| | Overview | Inputs | Secrets | Sandbox | Components       | |
| | --------                                                 | |
| |                                                          | |
| | This application deploys a complete web stack            | |
| | including a load balancer and managed database.          | |
| |                                                          | |
| | ## Features                                              | |
| | - Automatic scaling based on traffic                     | |
| | - Managed PostgreSQL with daily backups                  | |
| +----------------------------------------------------------+ |
|                                                              |
+--------------------------------------------------------------+
```

### 2. App Detail Page

```
         1111111111222222222233333333334444444444555555555566666
1234567890123456789012345678901234567890123456789012345678901234
+--------------------------------------------------------------+
| <- Back to catalog                                           |
| App Catalog                                 [ Install ]      |
+--------------------------------------------------------------+
|                                                              |
| +----------------------------------------------------------+ |
| | [logo]  My Application                         [AWS]     | |
| |                                                          | |
| | Overview | Inputs | Secrets | Sandbox | Components       | |
| | --------                                                 | |
| |                                                          | |
| | (active tab content rendered here)                       | |
| |                                                          | |
| +----------------------------------------------------------+ |
|                                                              |
+--------------------------------------------------------------+
```

### 3. Configure (Step 1) -- Not Logged In

```
         1111111111222222222233333333334444444444555555555566666
1234567890123456789012345678901234567890123456789012345678901234
+--------------------------------------------------------------+
|                                                              |
| [*] Configure  [ ] Stack  [ ] Sandbox  [ ] Components        |
|                                                              |
| [logo] My App              [ Cancel ] [ Deploy stack > ]     |
|                                                (disabled)    |
|                                                              |
| +----------------------------------------------------------+ |
| | Sign up or log in to continue                            | |
| |                                                          | |
| | [                   Sign up                   ]          | |
| |                                                          | |
| | -------------------- or --------------------             | |
| |                                                          | |
| | [                   Log in                    ]          | |
| +----------------------------------------------------------+ |
|                                                              |
| +----------------------------------------------------------+ |
| | Install Name                           (disabled)        | |
| | [ my-install                                    ]        | |
| +----------------------------------------------------------+ |
|                                                              |
| +----------------------------------------------------------+ |
| | Deployment Region                      (disabled)        | |
| | [ Loading...                                    ]        | |
| +----------------------------------------------------------+ |
|                                                              |
+--------------------------------------------------------------+
```

### 3b. Configure (Step 1) -- Logged In

```
         1111111111222222222233333333334444444444555555555566666
1234567890123456789012345678901234567890123456789012345678901234
+--------------------------------------------------------------+
|                                                              |
| [*] Configure  [ ] Stack  [ ] Sandbox  [ ] Components        |
|                                                              |
| [logo] My App              [ Cancel ] [ Deploy stack > ]     |
|                                                              |
| +----------------------------------------------------------+ |
| | Install Name                                             | |
| | [ my-install                                    ]        | |
| +----------------------------------------------------------+ |
|                                                              |
| +----------------------------------------------------------+ |
| | Deployment Region                                        | |
| | [ US East (N. Virginia) - us-east-1         v ]          | |
| +----------------------------------------------------------+ |
|                                                              |
| +----------------------------------------------------------+ |
| | Database Settings                                        | |
| |                                                          | |
| | +------------------------+ +------------------------+    | |
| | | DB Engine *            | | Replica Count          |    | |
| | | [ postgres       ]     | | [ 3              ]     |    | |
| | | Choose your engine     | | Number of replicas     |    | |
| | +------------------------+ +------------------------+    | |
| |                                                          | |
| | +------------------------+ +------------------------+    | |
| | | Storage (GB)           | | [x] Enable backups     |    | |
| | | [ 100            ]     | | Daily snapshots        |    | |
| | | Min: 20, Max: 1000     | |                        |    | |
| | +------------------------+ +------------------------+    | |
| +----------------------------------------------------------+ |
|                                                              |
+--------------------------------------------------------------+
```

### 4. Wizard -- Stack (Step 2)

```
         1111111111222222222233333333334444444444555555555566666
1234567890123456789012345678901234567890123456789012345678901234
+--------------------------------------------------------------+
|                                                              |
| [v] Configure  [*] Stack  [ ] Sandbox  [ ] Components        |
|                                                              |
| Stack Setup                      [ Deploy sandbox > ]        |
|                                   (disabled until done)      |
|                                                              |
| +----------------------------------------------------------+ |
| | 1. Setup your install stack                              | |
| |                                                          | |
| | [ Quick launch in AWS console -> ]                       | |
| |                                                          | |
| | CloudFormation template                                  | |
| | +----------------------------------------------------+   | |
| | | https://nuon-artifacts.s3.amazonaws.com/...        |   | |
| | +----------------------------------------------------+   | |
| |                                                          | |
| | ------------------- or -------------------               | |
| |                                                          | |
| | 2. Deploy with AWS CLI                                   | |
| |                                                          | |
| | Create stack                                             | |
| | +----------------------------------------------------+   | |
| | | aws cloudformation create-stack ...                |   | |
| | +----------------------------------------------------+   | |
| |                                                          | |
| | 3. Verify your stack                                     | |
| | [ Open in AWS console -> ]                               | |
| +----------------------------------------------------------+ |
|                                                              |
| +----------------------------------------------------------+ |
| | Stack Setup (2/4)                                        | |
| |                                                          | |
| | [v] Create CloudFormation stack                          | |
| | [v] Register runner                                      | |
| | [*] Waiting for runner heartbeat...                      | |
| | [ ] Verify runner health                                 | |
| +----------------------------------------------------------+ |
|                                                              |
+--------------------------------------------------------------+
```

### 5. Wizard -- Sandbox (Step 3)

```
         1111111111222222222233333333334444444444555555555566666
1234567890123456789012345678901234567890123456789012345678901234
+--------------------------------------------------------------+
|                                                              |
| [v] Configure  [v] Stack  [*] Sandbox  [ ] Components        |
|                                                              |
| Sandbox Provision    [Approve all] [Deploy components >]     |
|                                     (disabled until done)    |
|                                                              |
| +----------------------------------------------------------+ |
| | Provision plan   Awaiting Approval     [ Approve ]       | |
| |                                                          | |
| | Policy reports  |  Resource changes                      | |
| | ---------------                                          | |
| | v5  !2  x0                                               | |
| |                                                          | |
| | [v] security-baseline                                    | |
| | [v] cost-controls                                        | |
| | [!] encryption-policy                                    | |
| |     Warning: S3 bucket missing encryption                | |
| | [!] naming-convention                                    | |
| |     Warning: Resource missing required tags              | |
| | [v] network-policy                                       | |
| +----------------------------------------------------------+ |
|                                                              |
| +----------------------------------------------------------+ |
| | Sandbox (2/4)                                            | |
| |                                                          | |
| | [v] Plan approval                                        | |
| | [*] Apply sandbox provision...                           | |
| | [ ] Post-deployment validation                           | |
| | [ ] Health check                                         | |
| +----------------------------------------------------------+ |
|                                                              |
+--------------------------------------------------------------+
```

### 6. Wizard -- Components (Step 4)

```
         1111111111222222222233333333334444444444555555555566666
1234567890123456789012345678901234567890123456789012345678901234
+--------------------------------------------------------------+
|                                                              |
| [v] Configure  [v] Stack  [v] Sandbox  [*] Components        |
|                                                              |
| Component Deployment               [ View Overview > ]       |
|                                     (disabled until done)    |
|                                                              |
| +----------------------------------------------------------+ |
| | Provision plan   Awaiting Approval     [ Approve ]       | |
| |                                                          | |
| | Policy reports  |  Resource changes                      | |
| |                    ----------------                      | |
| |                                                          | |
| | + aws_ecs_service.api                                    | |
| | + aws_ecs_task_definition.api                            | |
| | + aws_lb_target_group.api                                | |
| | ~ aws_security_group.ecs                                 | |
| | + aws_rds_cluster.postgres                               | |
| | + aws_elasticache_cluster.redis                          | |
| +----------------------------------------------------------+ |
|                                                              |
| +----------------------------------------------------------+ |
| | api-server (3/5)                                         | |
| |                                                          | |
| | [v] Plan infrastructure                                  | |
| | [v] Policy check                                         | |
| | [*] Apply infrastructure...                              | |
| | [ ] Deploy container                                     | |
| | [ ] Health check                                         | |
| +----------------------------------------------------------+ |
|                                                              |
| +----------------------------------------------------------+ |
| | worker (1/5)                                             | |
| |                                                          | |
| | [*] Plan infrastructure...                               | |
| | [ ] Policy check                                         | |
| | [ ] Apply infrastructure                                 | |
| | [ ] Deploy container                                     | |
| | [ ] Health check                                         | |
| +----------------------------------------------------------+ |
|                                                              |
+--------------------------------------------------------------+
```

### 7. Install Overview

```
         1111111111222222222233333333334444444444555555555566666
1234567890123456789012345678901234567890123456789012345678901234
+--------------------------------------------------------------+
| [house]  Overview            [Reprovision] [Deprovision]     |
+--------------------------------------------------------------+
|                                                              |
| +---------------------------+ +---------------------------+  |
| | Stack                     | | Sandbox                   |  |
| |                           | |                           |  |
| | Status:  Completed        | | Status:  Completed        |  |
| | Region:  us-east-1        | | Repo:    acme/infra       |  |
| | Account: 123...890        | | Branch:  main             |  |
| +---------------------------+ +---------------------------+  |
|                                                              |
| +----------------------------------------------------------+ |
| | Components                                               | |
| | [api-server] [worker] [postgres] [redis] [nginx]         | |
| +----------------------------------------------------------+ |
|                                                              |
| History  |  Inputs                                           |
| -------                                                      |
|                                                              |
| +----------------------------------------------------------+ |
| | Type         Status        Date           Duration       | |
| +----------------------------------------------------------+ |
| | Provision    [v] Completed Nov 15, 2024   15m 32s        | |
| | Reprovision  [v] Completed Nov 20, 2024    8m 12s        | |
| | Reprovision  [x] Failed    Nov 22, 2024   22m 05s        | |
| +----------------------------------------------------------+ |
|                                                              |
+--------------------------------------------------------------+

Clicking a workflow row opens a slide-out panel:

+------------------------------+-------------------------------+
| (main content, narrowed)     | Provision          [<>] [X]   |
|                              |                               |
|                              | Status: Completed             |
|                              | Date:   Nov 15, 2024          |
|                              | Duration: 15m 32s             |
|                              |                               |
|                              | Stack (4/4)                   |
|                              | [v] Create CF stack           |
|                              | [v] Register runner           |
|                              | [v] Runner heartbeat          |
|                              | [v] Verify health             |
|                              |                               |
|                              | Sandbox (4/4)                 |
|                              | [v] Plan approval             |
|                              | [v] Apply provision           |
|                              | [v] Post-deploy validation    |
|                              | [v] Health check              |
|                              |                               |
|                              | Components (5/5)              |
|                              | [v] api-server                |
|                              | [v] worker                    |
|                              | [v] postgres                  |
|                              | [v] redis                     |
|                              | [v] nginx                     |
+------------------------------+-------------------------------+
```

### Legend

```
[v]  = completed (green checkmark)
[*]  = in progress (spinner/active icon)
[!]  = warning (amber)
[x]  = error/failed (red)
[ ]  = upcoming/pending (grey)
[<>] = expand/collapse panel toggle
->   = link/navigation arrow
v    = dropdown
```
