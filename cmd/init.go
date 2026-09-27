package cmd

import (
	"github.com/spf13/cobra"
)

// sanityIntegrationReplacedMessage is the whole output of `revkeen init`, and
// of `revkeen doctor --cms`. The Sanity Cart integration they served was
// removed in REV-8396 and replaced by RevKeen Storefront. The command and the
// --cms flag are kept for one release so existing scripts fail loudly instead
// of with "unknown command"; a follow-up ticket removes both.
const sanityIntegrationReplacedMessage = "The Sanity integration was replaced by RevKeen Storefront"

// sanityReplacedError carries the user-facing sentence verbatim. It is a
// named type rather than errors.New because the message is a full sentence
// printed as-is by main.go, and Go error-string style (ST1005) is for errors
// that get wrapped into other messages.
type sanityReplacedError struct{}

func (sanityReplacedError) Error() string { return sanityIntegrationReplacedMessage }

var errSanityIntegrationReplaced error = sanityReplacedError{}

func newInitCmd() *cobra.Command {
	var cmsTarget string
	var legacyString string
	var legacyBool bool

	cmd := &cobra.Command{
		Use:           "init",
		Short:         "Removed: the Sanity integration was replaced by RevKeen Storefront",
		Args:          cobra.ArbitraryArgs,
		Hidden:        true,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errSanityIntegrationReplaced
		},
	}

	cmd.Flags().StringVar(&cmsTarget, "cms", "", "Removed. Any value exits with an error.")
	// Accept the flags the removed command took, so an old invocation reaches
	// the replacement message rather than a flag-parsing error.
	for _, name := range []string{"framework", "deploy-target", "deploy", "project-path", "path"} {
		cmd.Flags().StringVar(&legacyString, name, "", "Removed.")
		_ = cmd.Flags().MarkHidden(name)
	}
	for _, name := range []string{"dry-run", "yes"} {
		cmd.Flags().BoolVar(&legacyBool, name, false, "Removed.")
		_ = cmd.Flags().MarkHidden(name)
	}

	return cmd
}
