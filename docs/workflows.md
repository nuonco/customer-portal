# Workflow Step Grouping

The ctl-api groups workflow steps together using two fields on `WorkflowStep`:

- **`group_idx`** (integer) — groups steps that belong to the same logical group (e.g., plan + apply steps share the same `group_idx`)
- **`group_retry_idx`** (integer) — counter for retries attempted on a group (defaults to 0, incremented on reruns)

## Example: Provision Workflow

From workflow `inwhtydyt1kcl9fbcrz4e5o475`:

| Group | Steps | Description |
|-------|-------|-------------|
| **1** | generate install state | Single step |
| **2** | provision runner service account | Single step |
| **3** | generate install stack, await install stack, update install stack outputs | Multi-step group |
| **4** | await runner health | Single step |
| **6** | provision sandbox plan, provision sandbox apply plan | Plan + apply pair |
| **8** | sync secrets | Single step |
| **10** | provision sandbox dns if enabled | Single step |
| **11** | await runner healthy | Single step |
| **13** | sync and plan certificate, apply certificate | Plan + apply pair |
| **14** | sync and plan whoami, apply whoami | Plan + apply pair |
| **15** | sync and plan ingress, apply ingress | Plan + apply pair |

Notes:
- Groups 5, 7, 9, 12 have no steps — the workflow skipped those indices.
- Plan + apply pairs share the same `group_idx` so they can be treated as a single logical operation.
- The `group_retry_idx` increments when a group is retried (e.g., stale plan detected).
