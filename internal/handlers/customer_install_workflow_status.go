package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon/sdks/nuon-go/models"
	"go.uber.org/zap"
)

// getStackSetupData checks for an active "await install stack" step and returns platform-specific setup data.
func (h *Handler) getStackSetupData(ctx context.Context, client *nuon.Client, install *models.Install, wf *nuonmodels.AppWorkflow) partials.StackSetupData {
	if wf.Steps == nil {
		return partials.StackSetupData{}
	}

	for _, step := range wf.Steps {
		if step.Name != "await install stack" {
			continue
		}
		// Determine platform from app runner type
		platform := h.detectPlatform(ctx, client, install)
		zap.L().Debug("getStackSetupData: detected platform", zap.String("platform", platform), zap.String("installID", install.NuonInstallID))

		stack, stackErr := client.GetInstallStack(ctx, install.NuonInstallID)
		if stackErr != nil {
			zap.L().Debug("getStackSetupData: failed to fetch install stack", zap.Error(stackErr))
		}

		switch platform {
		case "gcp":
			setup := partials.StackSetupData{
				Platform:      "gcp",
				NuonInstallID: install.NuonInstallID,
			}
			if stackErr == nil && stack != nil && stack.Versions != nil && len(stack.Versions) > 0 {
				setup.TfvarsContent = parseTfvars(stack.Versions[0].Contents)
				zap.L().Debug("getStackSetupData: parsed tfvars", zap.Bool("hasTfvars", setup.TfvarsContent != ""))
			} else {
				zap.L().Debug("getStackSetupData: no stack versions available for tfvars")
			}
			return setup

		default: // AWS and others use CloudFormation
			setup := partials.StackSetupData{Platform: platform}
			if stackErr == nil && stack != nil && stack.Versions != nil && len(stack.Versions) > 0 {
				v := stack.Versions[0]
				if v.QuickLinkURL != "" {
					setup.CloudFormationLink = v.QuickLinkURL
				}
				if v.TemplateURL != "" {
					setup.TemplateURL = v.TemplateURL
				}
				// Extract stack name from quick link URL, fallback to nuon-<installID>
				setup.StackName = "nuon-" + install.NuonInstallID
				if setup.CloudFormationLink != "" {
					if parsed, err := url.Parse(setup.CloudFormationLink); err == nil {
						// CF quick links use fragment params: ...#/stacks/create/review?stackName=...
						if fragment := parsed.Fragment; fragment != "" {
							if idx := strings.Index(fragment, "?"); idx >= 0 {
								if fq, err := url.ParseQuery(fragment[idx+1:]); err == nil {
									if sn := fq.Get("stackName"); sn != "" {
										setup.StackName = sn
									}
									if r := fq.Get("region"); r != "" {
										setup.Region = r
									}
								}
							}
						}
					}
				}
				if setup.Region == "" {
					setup.Region = install.Region
				}
				if setup.Region == "" {
					setup.Region = "us-east-1"
				}
			}
			// Append customer inputs to CF URL
			if setup.CloudFormationLink != "" {
				inputConfig, inputErr := client.GetAppInputConfig(ctx, install.GetAppID())
				if inputErr == nil && inputConfig != nil {
					var configMap map[string]interface{}
					jsonBytes, jerr := json.Marshal(inputConfig)
					if jerr == nil {
						if json.Unmarshal(jsonBytes, &configMap) == nil {
							inputMappings := extractCustomerInputMappings(configMap)
							if len(inputMappings) > 0 {
								currentInputs, ciErr := client.GetInstallCurrentInputs(ctx, install.NuonInstallID)
								if ciErr == nil && currentInputs != nil && currentInputs.Values != nil {
									setup.CloudFormationLink = appendInputsToCloudFormationURL(
										setup.CloudFormationLink,
										currentInputs.Values,
										inputMappings,
									)
								}
							}
						}
					}
				}
			}
			return setup
		}
	}
	return partials.StackSetupData{}
}

// detectPlatform determines the cloud platform from the app's runner type.
func (h *Handler) detectPlatform(ctx context.Context, client *nuon.Client, install *models.Install) string {
	appID := install.GetAppID()
	app, err := client.GetApp(ctx, appID)
	if err != nil {
		zap.L().Debug("detectPlatform: failed to fetch app", zap.String("appID", appID), zap.Error(err))
		return "aws" // default
	}
	if app == nil || app.RunnerConfig == nil {
		zap.L().Debug("detectPlatform: app or runner config is nil", zap.String("appID", appID))
		return "aws" // default
	}
	runnerType := string(app.RunnerConfig.AppRunnerType)
	zap.L().Debug("detectPlatform: runner type", zap.String("appID", appID), zap.String("runnerType", runnerType))
	switch runnerType {
	case "gcp":
		return "gcp"
	case "azure":
		return "azure"
	default:
		return "aws"
	}
}

// parseTfvars extracts the tfvars string from stack version contents.
// Contents may be a JSON string containing a "tfvars" key.
func parseTfvars(contents interface{}) string {
	if contents == nil {
		zap.L().Debug("parseTfvars: contents is nil")
		return ""
	}

	zap.L().Debug("parseTfvars: contents type", zap.String("type", fmt.Sprintf("%T", contents)))

	var raw interface{}
	switch v := contents.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &raw); err != nil {
			// Fallback: try base64 decode, then JSON unmarshal
			decoded, b64Err := base64.StdEncoding.DecodeString(v)
			if b64Err != nil {
				decoded, b64Err = base64.RawStdEncoding.DecodeString(v)
			}
			if b64Err != nil {
				zap.L().Debug("parseTfvars: failed to unmarshal or base64-decode string contents", zap.Error(err))
				return ""
			}
			if err := json.Unmarshal(decoded, &raw); err != nil {
				zap.L().Debug("parseTfvars: base64-decoded but failed to unmarshal JSON", zap.Error(err))
				return ""
			}
		}
	case map[string]interface{}:
		raw = v
	case json.RawMessage:
		if err := json.Unmarshal(v, &raw); err != nil {
			zap.L().Debug("parseTfvars: failed to unmarshal RawMessage contents", zap.Error(err))
			return ""
		}
	default:
		// Try JSON round-trip for unknown types
		b, err := json.Marshal(v)
		if err != nil {
			zap.L().Debug("parseTfvars: unsupported contents type, marshal failed", zap.String("type", fmt.Sprintf("%T", v)))
			return ""
		}
		if err := json.Unmarshal(b, &raw); err != nil {
			zap.L().Debug("parseTfvars: unsupported contents type, unmarshal failed", zap.String("type", fmt.Sprintf("%T", v)))
			return ""
		}
	}

	if m, ok := raw.(map[string]interface{}); ok {
		if tfvars, ok := m["tfvars"]; ok {
			zap.L().Debug("parseTfvars: found tfvars key")
			return fmt.Sprintf("%v", tfvars)
		}
		zap.L().Debug("parseTfvars: no tfvars key in contents map", zap.Int("numKeys", len(m)))
	}
	return ""
}

// findActiveStep returns the first non-completed step, or nil if all completed.
func findActiveStep(steps []*nuonmodels.AppWorkflowStep) *nuonmodels.AppWorkflowStep {
	for _, s := range steps {
		if s.Status != nil && (string(s.Status.Status) == "in-progress" || string(s.Status.Status) == "active") {
			return s
		}
	}
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Status != nil && string(steps[i].Status.Status) != "completed" {
			return steps[i]
		}
	}
	return nil
}

// isActiveWorkflowStatus returns true if the workflow status indicates it's still running.
func isActiveWorkflowStatus(status string) bool {
	return status == "in-progress" || status == "approval-awaiting" || status == "pending"
}

// isTerminalWorkflowStatus returns true if the workflow is in a final state.
func isTerminalWorkflowStatus(status string) bool {
	return status == "completed" || status == "success" || status == "error" || status == "cancelled"
}

// normalizeStepName lowercases and replaces spaces with underscores for comparison.
func normalizeStepName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "_")
}

// InstallReadmeStatus handles HTMX polling for readme display
