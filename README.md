# blockstm-sim

BlockSTM simulation and analysis tool for detecting concurrency anomalies in parallel transaction execution.

This tool is **chain-agnostic** — detection logic, harness, corpus, and reporter code live here as Altus Labs assets. Only minimal hook points (~150 LoC) are pushed upstream to `stablelabs/stable-sdk`.

## Prerequisites

- Go 1.25+
- Local clone of `stablelabs/stable-sdk`

## Setup

Clone and configure the `replace` directive to point at your local `stable-sdk`:

```bash
git clone git@github.com:altuslabsxyz/blockstm-sim.git
cd blockstm-sim

# Update replace path to your local stable-sdk clone
go mod edit -replace github.com/cosmos/cosmos-sdk=/path/to/your/stable-sdk
```

> **Note:** The `replace` directive in `go.mod` points to a local filesystem path. This is temporary while upstream PRs (PR-1 through PR-3a) are pending merge. Once merged, the replace will be removed in favor of a tagged version.

## Build

```bash
# Default build
make build

# With simulation harness shims enabled
make build-simharness

# With simulation harness + canary module
make build-canary
```

Or directly with `go build`:

```bash
# Default
go build -o build/blockstm-sim ./cmd/blockstm-sim

# With simharness tag
go build -tags simharness -o build/blockstm-sim ./cmd/blockstm-sim

# With simharness + canary tags
go build -tags "simharness simharness_canary" -o build/blockstm-sim ./cmd/blockstm-sim
```

## Build Tags

| Tag | Description |
|---|---|
| `simharness` | Enables BlockSTM simulation shims for intercepting transaction execution |
| `simharness_canary` | Includes canary module for experimental detection strategies (requires `simharness`) |

## Verify

```bash
./build/blockstm-sim version
./build/blockstm-sim version --long
./build/blockstm-sim version --long -o json
```

## Test

```bash
make test
```

## stable-sdk Replace Directive

The `go.mod` replace directive maps `github.com/cosmos/cosmos-sdk` to a local clone of `stablelabs/stable-sdk`. This is required because the hook-point PRs have not yet been merged upstream.

**Important:**
- Do **not** commit your local filesystem path to `go.mod` on `main`. CI overrides it automatically.
- Once the upstream PRs are merged, the replace directive will be removed.
- If `go mod tidy` fails, ensure your local `stable-sdk` is on the correct branch with the hook-point patches applied.
