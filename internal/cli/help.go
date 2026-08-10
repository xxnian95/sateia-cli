package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	commandRiskAnnotation = "sateia.io/risk"

	requiredTogetherAnnotation  = "cobra_annotation_required_if_others_set"
	oneRequiredAnnotation       = "cobra_annotation_one_required"
	mutuallyExclusiveAnnotation = "cobra_annotation_mutually_exclusive"
)

const (
	riskNone        = "NONE"
	riskReadOnly    = "READ_ONLY"
	riskLocalWrite  = "LOCAL_WRITE"
	riskWrite       = "WRITE"
	riskSoftDelete  = "SOFT_DELETE"
	riskLocalDelete = "LOCAL_DELETE"
)

type helpContract struct {
	SchemaVersion string           `json:"schema_version"`
	Command       string           `json:"command"`
	Use           string           `json:"use"`
	Short         string           `json:"short"`
	Long          string           `json:"long,omitempty"`
	Risk          string           `json:"risk"`
	Arguments     string           `json:"arguments,omitempty"`
	Examples      string           `json:"examples,omitempty"`
	Flags         []helpFlag       `json:"flags"`
	Rules         []helpRule       `json:"rules"`
	Subcommands   []helpSubcommand `json:"subcommands"`
}

type helpFlag struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     string `json:"default"`
	NoOptValue  string `json:"no_opt_value,omitempty"`
	Description string `json:"description"`
	Inherited   bool   `json:"inherited"`
}

type helpRule struct {
	Kind   string   `json:"kind"`
	Fields []string `json:"fields"`
}

type helpSubcommand struct {
	Name  string `json:"name"`
	Short string `json:"short"`
	Risk  string `json:"risk"`
}

type helpFormatValue string

func (value *helpFormatValue) Set(raw string) error {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized != "text" && normalized != "json" {
		return errors.New("must be text or json")
	}
	*value = helpFormatValue(normalized)
	return nil
}

func (value *helpFormatValue) String() string {
	return string(*value)
}

func (*helpFormatValue) Type() string {
	return "string"
}

func configureHelpContract(root *cobra.Command, format *helpFormatValue) {
	textHelp := root.HelpFunc()
	root.SetHelpFunc(func(command *cobra.Command, args []string) {
		switch string(*format) {
		case "", "text":
			textHelp(command, args)
		case "json":
			if err := writeHelpContract(command); err != nil {
				fmt.Fprintf(command.ErrOrStderr(), "Error: encode JSON help: %v\n", err)
			}
		}
	})
}

func writeHelpContract(command *cobra.Command) error {
	contract := buildHelpContract(command)
	encoder := json.NewEncoder(command.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(contract)
}

func buildHelpContract(command *cobra.Command) helpContract {
	contract := helpContract{
		SchemaVersion: "1",
		Command:       command.CommandPath(),
		Use:           command.UseLine(),
		Short:         command.Short,
		Long:          strings.TrimSpace(command.Long),
		Risk:          commandRisk(command),
		Arguments:     positionalArguments(command),
		Examples:      strings.TrimSpace(command.Example),
		Flags:         []helpFlag{},
		Rules:         []helpRule{},
		Subcommands:   []helpSubcommand{},
	}

	localFlags := map[string]bool{}
	command.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
		localFlags[flag.Name] = true
	})
	command.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		localFlags[flag.Name] = true
	})

	seenFlags := map[string]bool{}
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		contract.Flags = append(contract.Flags, helpFlagContract(flag, !localFlags[flag.Name]))
		seenFlags[flag.Name] = true
		contract.Rules = appendFlagRules(contract.Rules, flag)
	})
	command.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if seenFlags[flag.Name] {
			return
		}
		contract.Flags = append(contract.Flags, helpFlagContract(flag, true))
		contract.Rules = appendFlagRules(contract.Rules, flag)
	})

	sort.Slice(contract.Flags, func(left, right int) bool {
		return contract.Flags[left].Name < contract.Flags[right].Name
	})
	contract.Rules = uniqueHelpRules(contract.Rules)

	for _, child := range command.Commands() {
		if !child.IsAvailableCommand() || child.Hidden {
			continue
		}
		contract.Subcommands = append(contract.Subcommands, helpSubcommand{
			Name: child.Name(), Short: child.Short, Risk: commandRisk(child),
		})
	}
	sort.Slice(contract.Subcommands, func(left, right int) bool {
		return contract.Subcommands[left].Name < contract.Subcommands[right].Name
	})
	return contract
}

func helpFlagContract(flag *pflag.Flag, inherited bool) helpFlag {
	_, required := flag.Annotations[cobra.BashCompOneRequiredFlag]
	return helpFlag{
		Name: flag.Name, Shorthand: flag.Shorthand, Type: flag.Value.Type(), Required: required,
		Default: flag.DefValue, NoOptValue: flag.NoOptDefVal, Description: flag.Usage, Inherited: inherited,
	}
}

func appendFlagRules(rules []helpRule, flag *pflag.Flag) []helpRule {
	for annotation, kind := range map[string]string{
		requiredTogetherAnnotation:  "ALL_OR_NONE",
		oneRequiredAnnotation:       "AT_LEAST_ONE",
		mutuallyExclusiveAnnotation: "MUTUALLY_EXCLUSIVE",
	} {
		for _, group := range flag.Annotations[annotation] {
			fields := strings.Fields(group)
			sort.Strings(fields)
			rules = append(rules, helpRule{Kind: kind, Fields: fields})
		}
	}
	return rules
}

func uniqueHelpRules(rules []helpRule) []helpRule {
	unique := make(map[string]helpRule, len(rules))
	for _, rule := range rules {
		key := rule.Kind + ":" + strings.Join(rule.Fields, ",")
		unique[key] = rule
	}
	result := make([]helpRule, 0, len(unique))
	for _, rule := range unique {
		result = append(result, rule)
	}
	sort.Slice(result, func(left, right int) bool {
		leftKey := result[left].Kind + ":" + strings.Join(result[left].Fields, ",")
		rightKey := result[right].Kind + ":" + strings.Join(result[right].Fields, ",")
		return leftKey < rightKey
	})
	return result
}

func positionalArguments(command *cobra.Command) string {
	fields := strings.Fields(command.Use)
	if len(fields) < 2 {
		return ""
	}
	return strings.Join(fields[1:], " ")
}

func commandRisk(command *cobra.Command) string {
	if command.Annotations != nil {
		if risk := command.Annotations[commandRiskAnnotation]; risk != "" {
			return risk
		}
	}
	return riskNone
}

func setCommandRisk(command *cobra.Command, risk string) {
	if command.Annotations == nil {
		command.Annotations = map[string]string{}
	}
	command.Annotations[commandRiskAnnotation] = risk
}
