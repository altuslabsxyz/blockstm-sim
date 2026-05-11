package lint

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Scan keeper source for out-of-KVStore state mutations",
		Long: `Static analysis of Go keeper files to detect assignments to keeper struct
fields that bypass the KVStore wrapper, and writes to package-level variables
inside context-bearing methods. Both patterns cause BlockSTM safety violations.

Findings are heuristic: fields with conventionally immutable names (storeKey,
cdc, authority, etc.) are suppressed. All remaining assignments should be
manually reviewed to confirm whether the field is KVStore-backed or not.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sdkPath, _ := cmd.Flags().GetString("sdk-path")
			format, _ := cmd.Flags().GetString("format")
			kind, _ := cmd.Flags().GetString("kind")
			failOnFindings, _ := cmd.Flags().GetBool("fail-on-findings")

			scanner := NewScanner()
			result, err := scanner.ScanDir(sdkPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "lint: %v\n", err)
				os.Exit(1)
			}

			if kind != "" {
				k := Kind(kind)
				filtered := result.Findings[:0]
				for _, f := range result.Findings {
					if f.Kind == k {
						filtered = append(filtered, f)
					}
				}
				result.Findings = filtered
			}

			sort.Slice(result.Findings, func(i, j int) bool {
				a, b := result.Findings[i], result.Findings[j]
				if a.File != b.File {
					return a.File < b.File
				}
				return a.Line < b.Line
			})

			switch format {
			case "json":
				printJSON(result)
			default:
				printText(result, sdkPath)
			}

			if failOnFindings && len(result.Findings) > 0 {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().String("sdk-path", ".", "Path to SDK or module source tree to scan")
	cmd.Flags().String("format", "text", "Output format: text or json")
	cmd.Flags().String("kind", "", "Filter by kind: keeper_field or pkg_var")
	cmd.Flags().Bool("fail-on-findings", false, "Exit 1 if any findings are reported")

	return cmd
}

func printText(result *LintResult, sdkPath string) {
	fmt.Printf("blockstm-sim lint — scanned %d files in %s\n\n", result.Files, sdkPath)
	if len(result.Findings) == 0 {
		fmt.Println("No findings.")
		return
	}
	for _, f := range result.Findings {
		fmt.Printf("[%s] %s:%d  %s()  →  %s\n", f.Kind, f.File, f.Line, f.Method, f.Target)
	}
	fmt.Printf("\n%d finding(s)\n", len(result.Findings))
}

func printJSON(result *LintResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}
