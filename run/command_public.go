//go:build !sdk_hooks

package run

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run fixture corpus and report comparison results",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("simulation execution requires an SDK hook-enabled build")
		},
	}
}
