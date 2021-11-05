// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func newDestroyCmd() *cobra.Command {
	var debug bool
	var stack string

	var message string
	var execKind string
	var execAgent string

	// Flags for engine.UpdateOptions.
	var jsonDisplay bool
	var diffDisplay bool
	var eventLogPath string
	var parallel int
	var refresh string
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var skipPreview bool
	var suppressOutputs bool
	var suppressPermalink string
	var yes bool
	var targets *[]string
	var targetDependents bool
	var excludeProtected bool

	var cmd = &cobra.Command{
		Use:        "destroy",
		SuggestFor: []string{"delete", "down", "kill", "remove", "rm", "stop"},
		Short:      "Destroy an existing stack and its resources",
		Long: "Destroy an existing stack and its resources\n" +
			"\n" +
			"This command deletes an entire existing stack by name.  The current state is\n" +
			"loaded from the associated state file in the workspace.  After running to completion,\n" +
			"all of this stack's resources and associated state will be gone.\n" +
			"\n" +
			"Warning: this command is generally irreversible and should be used with great care.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			yes = yes || skipConfirmations()
			interactive := cmdutil.Interactive()
			if !interactive && !yes {
				return result.FromError(errors.New("--yes must be passed in to proceed when running in non-interactive mode"))
			}

			opts, err := updateFlagsToOptions(interactive, skipPreview, yes)
			if err != nil {
				return result.FromError(err)
			}

			var displayType = display.DisplayProgress
			if diffDisplay {
				displayType = display.DisplayDiff
			}

			opts.Display = display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				SuppressOutputs:      suppressOutputs,
				IsInteractive:        interactive,
				Type:                 displayType,
				EventLogPath:         eventLogPath,
				Debug:                debug,
				JSONDisplay:          jsonDisplay,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			//nolint:goconst
			if suppressPermalink == "true" {
				opts.Display.SuppressPermalink = true
			} else {
				opts.Display.SuppressPermalink = false
			}

			filestateBackend, err := isFilestateBackend(opts.Display)
			if err != nil {
				return result.FromError(err)
			}

			// by default, we are going to suppress the permalink when using self-managed backends
			// this can be re-enabled by explicitly passing "false" to the `suppress-permalink` flag
			//nolint:goconst
			if suppressPermalink != "false" && filestateBackend {
				opts.Display.SuppressPermalink = true
			}

			s, err := requireStack(stack, false, opts.Display, false /*setCurrent*/)
			if err != nil {
				return result.FromError(err)
			}
			proj, root, err := readProject()
			if err != nil {
				return result.FromError(err)
			}

			m, err := getUpdateMetadata(message, root, execKind, execAgent)
			if err != nil {
				return result.FromError(errors.Wrap(err, "gathering environment metadata"))
			}

			sm, err := getStackSecretsManager(s)
			if err != nil {
				return result.FromError(errors.Wrap(err, "getting secrets manager"))
			}

			cfg, err := getStackConfiguration(s, sm)
			if err != nil {
				return result.FromError(errors.Wrap(err, "getting stack configuration"))
			}

			targetUrns := []resource.URN{}
			for _, t := range *targets {
				targetUrns = append(targetUrns, resource.URN(t))
			}

			refreshOption, err := getRefreshOption(proj, refresh)
			if err != nil {
				return result.FromError(err)
			}

			var protectedCount int
			if excludeProtected {
				snapshot, err := s.Snapshot(commandContext())
				if err != nil {
					return result.FromError(err)
				} else if snapshot == nil {
					return result.FromError(errors.New("Failed to find the stack snapshot. Are you in a stack?"))
				}
				var unprotected []*resource.State
				unprotected, protected := seperateProtected(snapshot.Resources)
				protectedCount = len(protected)
				if len(unprotected) == 0 && protectedCount > 0 {
					fmt.Printf("There were no unprotected resources to destroy. There are still %d"+
						" protected resources associated with this stack.\n", protectedCount)
					// We need to return now. Otherwise the update will conclude
					// we tried to destroy everything and error for trying to
					// destroy a protected resource.
					return nil
				}
				for _, r := range unprotected {
					targetUrns = append(targetUrns, r.URN)
				}
			}

			if targets != nil && len(*targets) > 0 && excludeProtected {
				return result.FromError(errors.New("You cannot specify --target and --exclude-protected"))
			}
			opts.Engine = engine.UpdateOptions{
				Parallel:                  parallel,
				Debug:                     debug,
				Refresh:                   refreshOption,
				DestroyTargets:            targetUrns,
				TargetDependents:          targetDependents,
				UseLegacyDiff:             useLegacyDiff(),
				DisableProviderPreview:    disableProviderPreview(),
				DisableResourceReferences: disableResourceReferences(),
				DisableOutputValues:       disableOutputValues(),
			}

			_, res := s.Destroy(commandContext(), backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               opts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				Scopes:             cancellationScopes,
			})
			if res == nil && protectedCount > 0 && !jsonDisplay {
				fmt.Printf("All unprotected resources were destroyed. There are still %d protected resources"+
					" associated with this stack.\n", protectedCount)
			} else if res == nil && len(*targets) == 0 && !jsonDisplay {
				fmt.Printf("The resources in the stack have been deleted, but the history and configuration "+
					"associated with the stack are still maintained. \nIf you want to remove the stack "+
					"completely, run 'pulumi stack rm %s'.\n", s.Ref())
			} else if res != nil && res.Error() == context.Canceled {
				return result.FromError(errors.New("destroy cancelled"))
			}
			return PrintEngineResult(res)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the destroy operation")

	targets = cmd.PersistentFlags().StringArrayP(
		"target", "t", []string{},
		"Specify a single resource URN to destroy. All resources necessary to destroy this target will also be destroyed."+
			" Multiple resources can be specified using: --target urn1 --target urn2")
	cmd.PersistentFlags().BoolVar(
		&targetDependents, "target-dependents", false,
		"Allows destroying of dependent targets discovered but not specified in --target list")
	cmd.PersistentFlags().BoolVar(&excludeProtected, "exclude-protected", false, "Do not destroy protected resources."+
		" Destroy all other resources.")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.Flags().BoolVarP(
		&jsonDisplay, "json", "j", false,
		"Serialize the destroy diffs, operations, and overall output as JSON")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", defaultParallel,
		"Allow P resource operations to run in parallel at once (1 for no parallelism). Defaults to unbounded.")
	cmd.PersistentFlags().StringVarP(
		&refresh, "refresh", "r", "",
		"Refresh the state of the stack's resources before this update")
	cmd.PersistentFlags().Lookup("refresh").NoOptDefVal = "true"
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that don't need to be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&skipPreview, "skip-preview", "f", false,
		"Do not perform a preview before performing the destroy")
	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")
	cmd.PersistentFlags().StringVar(
		&suppressPermalink, "suppress-permalink", "",
		"Suppress display of the state permalink")
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically approve and perform the destroy after previewing it")

	if hasDebugCommands() {
		cmd.PersistentFlags().StringVar(
			&eventLogPath, "event-log", "",
			"Log events to a file at this path")
	}

	// internal flags
	cmd.PersistentFlags().StringVar(&execKind, "exec-kind", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")
	cmd.PersistentFlags().StringVar(&execAgent, "exec-agent", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-agent")

	return cmd
}

// seperateProtected returns a list or unprotected and protected resources
// respectively. This allows us to safely destroy all resources in the
// unprotected list without invalidating any resource in the protected list.
// Protection is contravarient.
//
// A
// B: Parent = A
// C: Parent = A, Protect = True
//
// -->
//
// Unprotected: B
// Protected: A, C
//
// We rely on the fact that `resources` is topologically sorted with respect to
// its dependencies. This function understands that providers live outside this
// topological sort.
func seperateProtected(resources []*resource.State) (
	/*unprotected*/ []*resource.State /*protected*/, []*resource.State) {
	protectedProviders := make(map[string]struct{})

	urns := make(map[resource.URN]*node, len(resources))

	for _, resource := range resources {
		urns[resource.URN] = &node{resource.Protect, resource}
		if resource.Protect {
			markProtected(resource.URN, urns, protectedProviders)
		}
	}

	// This will only trigger if (urn, node) is a provider. The check is implicit
	// in the set lookup.
	for urn, node := range urns {
		asProvider := fmt.Sprintf("%s::%s", string(urn), string(node.resource.ID))
		if _, ok := protectedProviders[asProvider]; ok {
			markProtected(urn, urns, protectedProviders)
		}
	}

	unprotected := make([]*resource.State, 0)
	protected := make([]*resource.State, 0)
	for _, r := range urns {
		// Default providers do not have a reasonable place in the resource DAG.
		// We ignore them.
		if !r.protected {
			unprotected = append(unprotected, r.resource)
		} else {
			protected = append(protected, r.resource)
		}
	}
	return unprotected, protected
}

// Mark a resource and its parents as protected.
func markProtected(urn resource.URN, urns map[resource.URN]*node, protectedProviders map[string]struct{}) {
	r := urns[urn]
	for {
		r.protected = true
		protectedProviders[r.resource.Provider] = struct{}{}
		for _, dep := range r.resource.Dependencies {
			markProtected(dep, urns, protectedProviders)
		}

		// If p is already protected, we don't need to continue to traverse.
		// All nodes above p will have already been marked as protected.
		// This is a property of `resources` being topologically sorted.
		if p, ok := urns[r.resource.Parent]; ok && !p.protected {
			r = p
		} else {
			break
		}
	}
}

// We create a wrapper because we don't want to mutate the contents of
// `resources`.
type node struct {
	protected bool
	resource  *resource.State
}
