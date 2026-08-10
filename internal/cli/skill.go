package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/agentskill"
)

type skillOptions struct {
	target string
	force  bool
}

func (app *application) newSkillCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "skill",
		Short: "Manage the bundled use-sateia-cli agent skill",
		Long: `Check, install, or update the use-sateia-cli agent skill bundled with this
exact CLI build.

The default target is $CODEX_HOME/skills/use-sateia-cli, or
~/.codex/skills/use-sateia-cli when CODEX_HOME is unset. Managed installations
include a hash manifest. Update refuses to overwrite files modified after the
previous CLI installation unless --force is explicit.`,
	}
	setCommandRisk(command, riskNone)
	command.AddCommand(app.newSkillCheckCommand(), app.newSkillInstallCommand(), app.newSkillUpdateCommand())
	return command
}

func (app *application) newSkillCheckCommand() *cobra.Command {
	options := skillOptions{}
	command := &cobra.Command{
		Use:   "check",
		Short: "Compare the installed skill with this CLI bundle",
		Long: `Compare the installed use-sateia-cli skill and installation manifest with the
bundle embedded in this CLI. This command is read-only. Inspect state in JSON
output: CURRENT, MISSING, OUTDATED, MODIFIED, UNMANAGED, or INVALID.

AI agents: immediately before every check, run "sateia skill check --help"
again. Installed CLI updates may change these instructions.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := agentskill.Check(options.target)
			if err != nil {
				return fmt.Errorf("check bundled skill: %w", err)
			}
			return app.printSkillStatus(command, status, app.jsonOutput)
		},
	}
	setCommandRisk(command, riskReadOnly)
	addSkillCommonFlags(command, &options, false)
	return command
}

func (app *application) newSkillInstallCommand() *cobra.Command {
	options := skillOptions{}
	command := &cobra.Command{
		Use:   "install",
		Short: "Install the skill bundled with this CLI",
		Long: `Install the exact use-sateia-cli skill bundled with this CLI.

The target must be missing unless --force is supplied. --force may replace
SKILL.md, agents/openai.yaml, and the Sateia installation manifest in the exact
target directory; unrelated files are preserved.

AI agents: immediately before every install, run "sateia skill install --help"
again. Installed CLI updates may change these instructions.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := agentskill.Install(options.target, app.version, options.force)
			if err != nil {
				return fmt.Errorf("install bundled skill: %w", err)
			}
			return app.printSkillStatus(command, status, app.jsonOutput)
		},
	}
	setCommandRisk(command, riskLocalWrite)
	addSkillCommonFlags(command, &options, true)
	return command
}

func (app *application) newSkillUpdateCommand() *cobra.Command {
	options := skillOptions{}
	command := &cobra.Command{
		Use:   "update",
		Short: "Update an unmodified managed skill from this CLI",
		Long: `Update the installed skill to the exact bundle embedded in this CLI.

A missing target is installed. An outdated managed target is updated only when
its files still match the previous installation manifest. MODIFIED, UNMANAGED,
and INVALID targets fail closed unless --force is supplied.

AI agents: immediately before every update, run "sateia skill update --help"
again. Installed CLI updates may change these instructions.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := agentskill.Update(options.target, app.version, options.force)
			if err != nil {
				return fmt.Errorf("update bundled skill: %w", err)
			}
			return app.printSkillStatus(command, status, app.jsonOutput)
		},
	}
	setCommandRisk(command, riskLocalWrite)
	addSkillCommonFlags(command, &options, true)
	return command
}

func addSkillCommonFlags(command *cobra.Command, options *skillOptions, withForce bool) {
	command.Flags().StringVar(&options.target, "target", "", "exact use-sateia-cli skill directory; omit for the Codex default")
	if withForce {
		command.Flags().BoolVar(&options.force, "force", false, "overwrite managed skill files even when the target is modified or unmanaged")
	}
}

func (app *application) printSkillStatus(command *cobra.Command, status agentskill.Status, jsonOutput bool) error {
	if jsonOutput {
		return app.writeJSON(command.Context(), struct {
			Skill agentskill.Status `json:"skill"`
		}{Skill: status})
	}
	fmt.Fprintf(app.out, "Sateia agent skill.\nstate: %s\ntarget: %s\nbundle_hash: %s\n", status.State, status.Target, status.BundleHash)
	if status.InstalledHash != "" {
		fmt.Fprintf(app.out, "installed_hash: %s\n", status.InstalledHash)
	}
	if status.InstalledVersion != "" {
		fmt.Fprintf(app.out, "installed_cli_version: %s\n", status.InstalledVersion)
	}
	fmt.Fprintf(app.out, "message: %s\n", status.Message)
	return nil
}
