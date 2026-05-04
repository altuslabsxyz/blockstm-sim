# blockstm-sim

BlockSTM simulation and analysis tooling for detecting concurrency anomalies in
parallel transaction execution.

## Prerequisites

- Go 1.25+

## Setup

```bash
git clone git@github.com:altuslabsxyz/blockstm-sim.git
cd blockstm-sim
go test ./...
```

## Build

```bash
# Default public build
make build

# Build with SDK hook integration enabled
make build-simharness

# Build with SDK hook integration and the canary module enabled
make build-canary
```

Or directly with `go build`:

```bash
go build -o build/blockstm-sim ./cmd/blockstm-sim
go build -tags "sdk_hooks simharness" -o build/blockstm-sim ./cmd/blockstm-sim
go build -tags "sdk_hooks simharness simharness_canary" -o build/blockstm-sim ./cmd/blockstm-sim
```

## Build Tags

| Tag | Description |
|---|---|
| `sdk_hooks` | Enables SDK hook integration used by the simulation executor |
| `simharness` | Enables BlockSTM simulation shims for intercepting transaction execution |
| `simharness_canary` | Includes the canary module for experimental detection strategies |

## Verify

```bash
./build/blockstm-sim version
./build/blockstm-sim version --long
./build/blockstm-sim version --long -o json
```

## Test

```bash
make test
make test-canary
```

The default public build uses only upstream dependencies. SDK hook-enabled
builds are intended for internal CI environments that inject the required SDK
source through an untracked modfile.
