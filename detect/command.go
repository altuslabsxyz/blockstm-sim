package detect

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

type formatter interface {
	Header(sdkPath string)
	Finding(f Finding)
	Footer(result *ScanResult, sdkPath string)
}

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Scan SDK source for non-deterministic function calls",
		Long:  "Static analysis of Go source files to detect forbidden calls to time, rand, and I/O packages that break deterministic execution.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sdkPath, _ := cmd.Flags().GetString("sdk-path")
			category, _ := cmd.Flags().GetString("category")
			format, _ := cmd.Flags().GetString("format")
			excludePaths, _ := cmd.Flags().GetStringSlice("exclude-path")

			scanner := NewTypeScanner(DefaultRules())
			result, err := scanner.ScanDir(sdkPath, excludePaths...)
			if err != nil {
				return fmt.Errorf("scan: %w", err)
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

			var rep formatter
			switch format {
			case "json":
				rep = NewJSONReporter(os.Stdout)
			case "markdown":
				rep = NewMarkdownReporter(os.Stdout)
			default:
				rep = NewReporter(os.Stdout)
			}

			rep.Header(sdkPath)
			for _, f := range result.Findings {
				rep.Finding(f)
			}
			rep.Footer(result, sdkPath)

			if len(result.Findings) > 0 {
				return fmt.Errorf("%d finding(s) detected", len(result.Findings))
			}
			return nil
		},
	}

	cmd.Flags().String("sdk-path", "../cosmos-sdk", "Path to SDK source tree")
	cmd.Flags().String("category", "", "Filter to a single category: time, rand, or io")
	cmd.Flags().String("format", "text", "Output format: text, json, or markdown")
	cmd.Flags().StringSlice("exclude-path", nil, "Path prefixes to exclude (relative to sdk-path, repeatable). E.g. --exclude-path client/cli")

	return cmd
}
