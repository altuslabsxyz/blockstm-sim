package version

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	_ "github.com/cosmos/cosmos-sdk/version"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTags = ""
)

type Info struct {
	Version    string `json:"version" yaml:"version"`
	GitCommit  string `json:"commit" yaml:"commit"`
	BuildTags  string `json:"build_tags" yaml:"build_tags"`
	GoVersion  string `json:"go" yaml:"go"`
	SDKVersion string `json:"sdk_version" yaml:"sdk_version"`
}

func NewInfo() Info {
	return Info{
		Version:    Version,
		GitCommit:  Commit,
		BuildTags:  BuildTags,
		GoVersion:  fmt.Sprintf("go version %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
		SDKVersion: getSDKVersion(),
	}
}

func getSDKVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range bi.Deps {
		if dep.Path == "github.com/cosmos/cosmos-sdk" {
			if dep.Replace != nil {
				return fmt.Sprintf("%s (local replace)", dep.Replace.Path)
			}
			return dep.Version
		}
	}
	return "unknown"
}

func (vi Info) String() string {
	return fmt.Sprintf(`blockstm-sim %s
git commit: %s
build tags: %s
%s
cosmos-sdk: %s`, vi.Version, vi.GitCommit, vi.BuildTags, vi.GoVersion, vi.SDKVersion)
}

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			verInfo := NewInfo()

			long, _ := cmd.Flags().GetBool("long")
			if !long {
				fmt.Fprintln(cmd.OutOrStdout(), verInfo.Version)
				return nil
			}

			var (
				bz  []byte
				err error
			)

			output, _ := cmd.Flags().GetString("output")
			switch strings.ToLower(output) {
			case "json":
				bz, err = json.Marshal(verInfo)
			default:
				bz, err = yaml.Marshal(&verInfo)
			}
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(bz))
			return nil
		},
	}

	cmd.Flags().Bool("long", false, "Print long version information")
	cmd.Flags().StringP("output", "o", "text", "Output format (text|json)")

	return cmd
}
