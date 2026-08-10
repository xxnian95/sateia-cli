package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/agentskill"
	"github.com/xxnian95/sateia-cli/internal/api"
)

type doctorReport struct {
	Healthy bool          `json:"healthy"`
	Version string        `json:"version"`
	Checks  []doctorCheck `json:"checks"`
}

type doctorCheck struct {
	Name             string `json:"name"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	Server           string `json:"server,omitempty"`
	CredentialSource string `json:"credential_source,omitempty"`
	RequestID        string `json:"request_id,omitempty"`
	SkillState       string `json:"skill_state,omitempty"`
	SkillTarget      string `json:"skill_target,omitempty"`
	LatestVersion    string `json:"latest_version,omitempty"`
}

func (app *application) newDoctorCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose CLI, server, credential, update, and skill health",
		Long: `Run read-only diagnostics for the installed CLI version, local clock, selected
server, effective credential source, authenticated API access, update status,
and bundled agent skill installation. Tokens and credential contents are never
printed. The command reports healthy=false when a required check fails; warnings
such as a missing optional skill do not make the CLI unhealthy.

AI agents: immediately before every diagnostic run, execute
"sateia doctor --help" again. Installed CLI updates may change these checks.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			report := app.runDoctor(command)
			if app.jsonOutput {
				return app.writeJSON(command.Context(), report)
			}
			printDoctorReport(app.out, report)
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
	setCommandRisk(command, riskReadOnly)
	return command
}

func (app *application) runDoctor(command *cobra.Command) doctorReport {
	report := doctorReport{Healthy: true, Version: app.version, Checks: []doctorCheck{}}
	if app.version == "" || app.version == "dev" {
		report.Checks = append(report.Checks, doctorCheck{Name: "version", Status: "WARN", Message: "This is a development build without a stable release version."})
	} else {
		report.Checks = append(report.Checks, doctorCheck{Name: "version", Status: "PASS", Message: "CLI version is available."})
	}
	report.Checks = append(report.Checks, doctorCheck{
		Name: "clock", Status: "PASS", Message: "Local clock and UTC offset are available: " + time.Now().Format(time.RFC3339),
	})

	baseURL, _, err := app.baseURL()
	if err != nil {
		report.Healthy = false
		report.Checks = append(report.Checks, doctorCheck{Name: "server", Status: "FAIL", Message: err.Error()})
		report.Checks = append(report.Checks, doctorCheck{Name: "credential", Status: "SKIP", Message: "Skipped because server selection failed."})
		report.Checks = append(report.Checks, doctorCheck{Name: "authentication", Status: "SKIP", Message: "Skipped because server selection failed."})
	} else {
		report.Checks = append(report.Checks, doctorCheck{Name: "server", Status: "PASS", Message: "Server URL is valid.", Server: baseURL})
		token, source, tokenErr := app.token(baseURL)
		if tokenErr != nil {
			report.Healthy = false
			report.Checks = append(report.Checks, doctorCheck{Name: "credential", Status: "FAIL", Message: authenticationTokenError(tokenErr, source).Error(), CredentialSource: source})
			report.Checks = append(report.Checks, doctorCheck{Name: "authentication", Status: "SKIP", Message: "Skipped because the effective credential is unavailable."})
		} else {
			report.Checks = append(report.Checks, doctorCheck{Name: "credential", Status: "PASS", Message: "Effective credential is readable without exposing its secret.", CredentialSource: source})
			client, clientErr := api.NewClient(baseURL, token, app.version, nil)
			if clientErr != nil {
				report.Healthy = false
				report.Checks = append(report.Checks, doctorCheck{Name: "authentication", Status: "FAIL", Message: clientErr.Error()})
			} else {
				metadata, authErr := client.CheckAuthentication(command.Context())
				if authErr != nil {
					report.Healthy = false
					check := doctorCheck{Name: "authentication", Status: "FAIL", Message: authenticationCheckError(authErr, source).Error()}
					var apiErr *api.APIError
					if errors.As(authErr, &apiErr) {
						check.RequestID = apiErr.RequestID
					}
					report.Checks = append(report.Checks, check)
				} else {
					report.Checks = append(report.Checks, doctorCheck{Name: "authentication", Status: "PASS", Message: "Authenticated read-only API request succeeded.", RequestID: metadata.RequestID})
				}
			}
		}
	}

	skillStatus, skillErr := agentskill.Check("")
	if skillErr != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "skill", Status: "WARN", Message: skillErr.Error()})
	} else {
		status := "WARN"
		if skillStatus.State == agentskill.StateCurrent {
			status = "PASS"
		}
		report.Checks = append(report.Checks, doctorCheck{
			Name: "skill", Status: status, Message: skillStatus.Message,
			SkillState: string(skillStatus.State), SkillTarget: skillStatus.Target,
		})
	}

	if app.updateChecker == nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "update", Status: "WARN", Message: "Update checker is unavailable in this build."})
	} else if available, updateErr := app.updateChecker.Check(command.Context(), app.version); updateErr != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "update", Status: "WARN", Message: "Update check failed: " + updateErr.Error()})
	} else if available != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "update", Status: "WARN", Message: "A newer stable CLI version is available.", LatestVersion: available.LatestVersion})
	} else {
		report.Checks = append(report.Checks, doctorCheck{Name: "update", Status: "PASS", Message: "No newer stable CLI version was reported."})
	}
	return report
}

func printDoctorReport(output io.Writer, report doctorReport) {
	fmt.Fprintf(output, "Sateia doctor.\nhealthy: %t\nversion: %s\n", report.Healthy, report.Version)
	for _, check := range report.Checks {
		fmt.Fprintf(output, "\n[%s] %s: %s\n", check.Status, check.Name, check.Message)
		if check.Server != "" {
			fmt.Fprintf(output, "server: %s\n", check.Server)
		}
		if check.CredentialSource != "" {
			fmt.Fprintf(output, "credential_source: %s\n", check.CredentialSource)
		}
		if check.RequestID != "" {
			fmt.Fprintf(output, "request_id: %s\n", check.RequestID)
		}
		if check.SkillState != "" {
			fmt.Fprintf(output, "skill_state: %s\nskill_target: %s\n", check.SkillState, check.SkillTarget)
		}
		if check.LatestVersion != "" {
			fmt.Fprintf(output, "latest_version: %s\n", check.LatestVersion)
		}
	}
}
