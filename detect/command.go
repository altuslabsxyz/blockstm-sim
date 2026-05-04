package detect

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Scan SDK source for non-deterministic function calls",
		Long:  "Static analysis of Go source files to detect forbidden calls to time, rand, and I/O packages that break deterministic execution.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sdkPath, _ := cmd.Flags().GetString("sdk-path")
			category, _ := cmd.Flags().GetString("category")

			scanner := NewScanner(DefaultRules())
			result, err := scanner.ScanDir(sdkPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "scan: %v\n", err)
				os.Exit(1)
			}

			if category != "" {
				cat := Category(category)
				filtered := result.Findings[:0]
				for _, f := range result.Findings {
					if f.Category == cat {
						filtered = append(filtered, f)
					}
				}
				result.Findings = filtered
			}

			sort.Slice(result.Findings, func(i, j int) bool {
				if result.Findings[i].File != result.Findings[j].File {
					return result.Findings[i].File < result.Findings[j].File
				}
				return result.Findings[i].Line < result.Findings[j].Line
			})

			rep := NewReporter(os.Stdout)
			rep.Header(sdkPath)
			for _, f := range result.Findings {
				rep.Finding(f)
			}
			rep.Footer(result, sdkPath)

			if len(result.Findings) > 0 {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().String("sdk-path", "../stable-sdk", "Path to SDK source tree")
	cmd.Flags().String("category", "", "Filter to a single category: time, rand, or io")

	return cmd
}
