package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/cometbft/cometbft/abci/server"
	cmtcmd "github.com/cometbft/cometbft/cmd/cometbft/commands"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/crypto"
	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	pvm "github.com/cometbft/cometbft/privval"
	cmtstate "github.com/cometbft/cometbft/proto/tendermint/state"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/proxy"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cometbft/cometbft/rpc/client/local"
	sm "github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/store"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/hashicorp/go-metrics"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pruningtypes "cosmossdk.io/store/pruning/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server/api"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	servergrpc "github.com/cosmos/cosmos-sdk/server/grpc"
	servercmtlog "github.com/cosmos/cosmos-sdk/server/log"
	"github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/version"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
)

const (
	// CometBFT full-node start flags
	flagWithComet          = "with-comet"
	flagAddress            = "address"
	flagTransport          = "transport"
	flagTraceStore         = "trace-store"
	flagCPUProfile         = "cpu-profile"
	FlagMinGasPrices       = "minimum-gas-prices"
	FlagQueryGasLimit      = "query-gas-limit"
	FlagHaltHeight         = "halt-height"
	FlagHaltTime           = "halt-time"
	FlagInterBlockCache    = "inter-block-cache"
	FlagUnsafeSkipUpgrades = "unsafe-skip-upgrades"
	FlagTrace              = "trace"
	FlagInvCheckPeriod     = "inv-check-period"

	FlagPruning             = "pruning"
	FlagPruningKeepRecent   = "pruning-keep-recent"
	FlagPruningInterval     = "pruning-interval"
	FlagIndexEvents         = "index-events"
	FlagMinRetainBlocks     = "min-retain-blocks"
	FlagIAVLCacheSize       = "iavl-cache-size"
	FlagDisableIAVLFastNode = "iavl-disable-fastnode"
	FlagIAVLSyncPruning     = "iavl-sync-pruning"
	FlagShutdownGrace       = "shutdown-grace"

	// state sync-related flags
	FlagStateSyncSnapshotInterval   = "state-sync.snapshot-interval"
	FlagStateSyncSnapshotKeepRecent = "state-sync.snapshot-keep-recent"

	// api-related flags
	FlagAPIEnable             = "api.enable"
	FlagAPISwagger            = "api.swagger"
	FlagAPIAddress            = "api.address"
	FlagAPIMaxOpenConnections = "api.max-open-connections"
	FlagRPCReadTimeout        = "api.rpc-read-timeout"
	FlagRPCWriteTimeout       = "api.rpc-write-timeout"
	FlagRPCMaxBodyBytes       = "api.rpc-max-body-bytes"
	FlagAPIEnableUnsafeCORS   = "api.enabled-unsafe-cors"

	// gRPC-related flags
	flagGRPCOnly                        = "grpc-only"
	flagGRPCEnable                      = "grpc.enable"
	flagGRPCAddress                     = "grpc.address"
	flagGRPCWebEnable                   = "grpc-web.enable"
	flagGRPCSkipCheckHeader             = "grpc.skip-check-header"
	flagHistoricalGRPCAddressBlockRange = "grpc.historical-grpc-address-block-range"

	// mempool flags
	FlagMempoolMaxTxs = "mempool.max-txs"

	// testnet keys
	KeyIsTestnet             = "is-testnet"
	KeyNewChainID            = "new-chain-ID"
	KeyNewValAddr            = "new-validator-addr"
	KeyUserPubKey            = "user-pub-key"
	KeyTriggerTestnetUpgrade = "trigger-testnet-upgrade"
	KeyTestnetValidators     = "testnet-validators"
	KeyTestnetOutputDir      = "testnet-output-dir"
)

// StartCmdOptions defines options that can be customized in `StartCmdWithOptions`,
type StartCmdOptions struct {
	// DBOpener can be used to customize db opening, for example customize db options or support different db backends,
	// default to the builtin db opener.
	DBOpener func(rootDir string, backendType dbm.BackendType) (dbm.DB, error)
	// PostSetup can be used to setup extra services under the same cancellable context,
	// it's not called in stand-alone mode, only for in-process mode.
	PostSetup func(svrCtx *Context, clientCtx client.Context, ctx context.Context, g *errgroup.Group) error
	// PostSetupStandalone can be used to setup extra services under the same cancellable context,
	PostSetupStandalone func(svrCtx *Context, clientCtx client.Context, ctx context.Context, g *errgroup.Group) error
	// AddFlags add custom flags to start cmd
	AddFlags func(cmd *cobra.Command)
	// StartCommandHanlder can be used to customize the start command handler
	StartCommandHandler func(svrCtx *Context, clientCtx client.Context, appCreator types.AppCreator, inProcessConsensus bool, opts StartCmdOptions) error
	// TestnetConfigModifier is called for each exported node's app.toml during multi-validator export.
	// The app layer can use this to set testnet/mainnet-specific config values (e.g., evm-chain-id).
	// isTestnet is true when --trigger-testnet-upgrade contains "rc".
	TestnetConfigModifier func(v *viper.Viper, isTestnet bool, nodeIndex int)
}

// StartCmd runs the service passed in, either stand-alone or in-process with
// CometBFT.
func StartCmd(appCreator types.AppCreator, defaultNodeHome string) *cobra.Command {
	return StartCmdWithOptions(appCreator, defaultNodeHome, StartCmdOptions{})
}

// StartCmdWithOptions runs the service passed in, either stand-alone or in-process with
// CometBFT.
func StartCmdWithOptions(appCreator types.AppCreator, defaultNodeHome string, opts StartCmdOptions) *cobra.Command {
	if opts.DBOpener == nil {
		opts.DBOpener = openDB
	}

	if opts.StartCommandHandler == nil {
		opts.StartCommandHandler = start
	}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Run the full node",
		Long: `Run the full node application with CometBFT in or out of process. By
default, the application will run with CometBFT in process.

Pruning options can be provided via the '--pruning' flag or alternatively with '--pruning-keep-recent', and
'pruning-interval' together.

For '--pruning' the options are as follows:

default: the last 362880 states are kept, pruning at 10 block intervals
nothing: all historic states will be saved, nothing will be deleted (i.e. archiving node)
everything: 2 latest states will be kept; pruning at 10 block intervals.
custom: allow pruning options to be manually specified through 'pruning-keep-recent', and 'pruning-interval'

Node halting configurations exist in the form of two flags: '--halt-height' and '--halt-time'. During
the ABCI Commit phase, the node will check if the current block height is greater than or equal to
the halt-height or if the current block time is greater than or equal to the halt-time. If so, the
node will attempt to gracefully shutdown and the block will not be committed. In addition, the node
will not be able to commit subsequent blocks.

For profiling and benchmarking purposes, CPU profiling can be enabled via the '--cpu-profile' flag
which accepts a path for the resulting pprof file.

The node may be started in a 'query only' mode where only the gRPC and JSON HTTP
API services are enabled via the 'grpc-only' flag. In this mode, CometBFT is
bypassed and can be used when legacy queries are needed after an on-chain upgrade
is performed. Note, when enabled, gRPC will also be automatically enabled.
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			serverCtx := GetServerContextFromCmd(cmd)

			_, err := GetPruningOptionsFromFlags(serverCtx.Viper)
			if err != nil {
				return err
			}

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			withCMT, _ := cmd.Flags().GetBool(flagWithComet)
			if !withCMT {
				serverCtx.Logger.Info("starting ABCI without CometBFT")
			}

			err = wrapCPUProfile(serverCtx, func() error {
				return opts.StartCommandHandler(serverCtx, clientCtx, appCreator, withCMT, opts)
			})

			serverCtx.Logger.Debug("received quit signal")
			graceDuration, _ := cmd.Flags().GetDuration(FlagShutdownGrace)
			if graceDuration > 0 {
				serverCtx.Logger.Info("graceful shutdown start", FlagShutdownGrace, graceDuration)
				<-time.After(graceDuration)
				serverCtx.Logger.Info("graceful shutdown complete")
			}

			return err
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The application home directory")
	addStartNodeFlags(cmd, opts)
	return cmd
}

func start(svrCtx *Context, clientCtx client.Context, appCreator types.AppCreator, withCmt bool, opts StartCmdOptions) error {
	svrCfg, err := getAndValidateConfig(svrCtx)
	if err != nil {
		return err
	}

	app, appCleanupFn, err := startApp(svrCtx, appCreator, opts)
	if err != nil {
		return err
	}
	defer appCleanupFn()

	metrics, err := startTelemetry(svrCfg)
	if err != nil {
		return err
	}

	emitServerInfoMetrics()

	if !withCmt {
		return startStandAlone(svrCtx, svrCfg, clientCtx, app, metrics, opts)
	}
	return startInProcess(svrCtx, svrCfg, clientCtx, app, metrics, opts)
}

func startStandAlone(svrCtx *Context, svrCfg serverconfig.Config, clientCtx client.Context, app types.Application, metrics *telemetry.Metrics, opts StartCmdOptions) error {
	addr := svrCtx.Viper.GetString(flagAddress)
	transport := svrCtx.Viper.GetString(flagTransport)

	cmtApp := NewCometABCIWrapper(app)
	svr, err := server.NewServer(addr, transport, cmtApp)
	if err != nil {
		return fmt.Errorf("error creating listener: %w", err)
	}

	svr.SetLogger(servercmtlog.CometLoggerWrapper{Logger: svrCtx.Logger.With("module", "abci-server")})

	g, ctx := getCtx(svrCtx, false)

	// Add the tx service to the gRPC router. We only need to register this
	// service if API or gRPC is enabled, and avoid doing so in the general
	// case, because it spawns a new local CometBFT RPC client.
	if svrCfg.API.Enable || svrCfg.GRPC.Enable {
		// create tendermint client
		// assumes the rpc listen address is where tendermint has its rpc server
		rpcclient, err := rpchttp.New(svrCtx.Config.RPC.ListenAddress, "/websocket")
		if err != nil {
			return err
		}
		// re-assign for making the client available below
		// do not use := to avoid shadowing clientCtx
		clientCtx = clientCtx.WithClient(rpcclient)

		// use the provided clientCtx to register the services
		app.RegisterTxService(clientCtx)
		app.RegisterTendermintService(clientCtx)
		app.RegisterNodeService(clientCtx, svrCfg)
	}

	grpcSrv, clientCtx, err := StartGrpcServer(ctx, g, svrCfg.GRPC, clientCtx, svrCtx, app)
	if err != nil {
		return err
	}

	err = startAPIServer(ctx, g, svrCfg, clientCtx, svrCtx, app, svrCtx.Config.RootDir, grpcSrv, metrics)
	if err != nil {
		return err
	}

	if opts.PostSetupStandalone != nil {
		if err := opts.PostSetupStandalone(svrCtx, clientCtx, ctx, g); err != nil {
			return err
		}
	}

	g.Go(func() error {
		if err := svr.Start(); err != nil {
			svrCtx.Logger.Error("failed to start out-of-process ABCI server", "err", err)
			return err
		}

		// Wait for the calling process to be canceled or close the provided context,
		// so we can gracefully stop the ABCI server.
		<-ctx.Done()
		svrCtx.Logger.Info("stopping the ABCI server...")
		return errors.Join(svr.Stop(), app.Close())
	})

	return g.Wait()
}

func startInProcess(svrCtx *Context, svrCfg serverconfig.Config, clientCtx client.Context, app types.Application,
	metrics *telemetry.Metrics, opts StartCmdOptions,
) error {
	cmtCfg := svrCtx.Config
	gRPCOnly := svrCtx.Viper.GetBool(flagGRPCOnly)

	g, ctx := getCtx(svrCtx, true)

	if gRPCOnly {
		// TODO: Generalize logic so that gRPC only is really in startStandAlone
		svrCtx.Logger.Info("starting node in gRPC only mode; CometBFT is disabled")
		svrCfg.GRPC.Enable = true
	} else {
		svrCtx.Logger.Info("starting node with ABCI CometBFT in-process")
		tmNode, cleanupFn, err := startCmtNode(ctx, cmtCfg, app, svrCtx)
		if err != nil {
			return err
		}
		defer cleanupFn()

		// Add the tx service to the gRPC router. We only need to register this
		// service if API or gRPC is enabled, and avoid doing so in the general
		// case, because it spawns a new local CometBFT RPC client.
		if svrCfg.API.Enable || svrCfg.GRPC.Enable {
			// Re-assign for making the client available below do not use := to avoid
			// shadowing the clientCtx variable.
			clientCtx = clientCtx.WithClient(local.New(tmNode))

			app.RegisterTxService(clientCtx)
			app.RegisterTendermintService(clientCtx)
			app.RegisterNodeService(clientCtx, svrCfg)
		}
	}

	grpcSrv, clientCtx, err := StartGrpcServer(ctx, g, svrCfg.GRPC, clientCtx, svrCtx, app)
	if err != nil {
		return err
	}

	err = startAPIServer(ctx, g, svrCfg, clientCtx, svrCtx, app, cmtCfg.RootDir, grpcSrv, metrics)
	if err != nil {
		return err
	}

	if opts.PostSetup != nil {
		if err := opts.PostSetup(svrCtx, clientCtx, ctx, g); err != nil {
			return err
		}
	}

	// wait for signal capture and gracefully return
	// we are guaranteed to be waiting for the "ListenForQuitSignals" goroutine.
	return g.Wait()
}

// TODO: Move nodeKey into being created within the function.
func startCmtNode(
	ctx context.Context,
	cfg *cmtcfg.Config,
	app types.Application,
	svrCtx *Context,
) (tmNode *node.Node, cleanupFn func(), err error) {
	nodeKey, err := p2p.LoadOrGenNodeKey(cfg.NodeKeyFile())
	if err != nil {
		return nil, cleanupFn, err
	}

	cmtApp := NewCometABCIWrapper(app)
	tmNode, err = node.NewNodeWithContext(
		ctx,
		cfg,
		pvm.LoadOrGenFilePV(cfg.PrivValidatorKeyFile(), cfg.PrivValidatorStateFile()),
		nodeKey,
		proxy.NewLocalClientCreator(cmtApp),
		getGenDocProvider(cfg),
		cmtcfg.DefaultDBProvider,
		node.DefaultMetricsProvider(cfg.Instrumentation),
		servercmtlog.CometLoggerWrapper{Logger: svrCtx.Logger},
	)
	if err != nil {
		return tmNode, cleanupFn, err
	}

	if err := tmNode.Start(); err != nil {
		return tmNode, cleanupFn, err
	}

	cleanupFn = func() {
		if tmNode != nil && tmNode.IsRunning() {
			_ = tmNode.Stop()
			_ = app.Close()
		}
	}

	return tmNode, cleanupFn, nil
}

func getAndValidateConfig(svrCtx *Context) (serverconfig.Config, error) {
	config, err := serverconfig.GetConfig(svrCtx.Viper)
	if err != nil {
		return config, err
	}

	if err := config.ValidateBasic(); err != nil {
		return config, err
	}
	return config, nil
}

// returns a function which returns the genesis doc from the genesis file.
func getGenDocProvider(cfg *cmtcfg.Config) func() (*cmttypes.GenesisDoc, error) {
	return func() (*cmttypes.GenesisDoc, error) {
		appGenesis, err := genutiltypes.AppGenesisFromFile(cfg.GenesisFile())
		if err != nil {
			return nil, err
		}

		return appGenesis.ToGenesisDoc()
	}
}

func setupTraceWriter(svrCtx *Context) (traceWriter io.WriteCloser, cleanup func(), err error) {
	// clean up the traceWriter when the server is shutting down
	cleanup = func() {}

	traceWriterFile := svrCtx.Viper.GetString(flagTraceStore)
	traceWriter, err = openTraceWriter(traceWriterFile)
	if err != nil {
		return traceWriter, cleanup, err
	}

	// if flagTraceStore is not used then traceWriter is nil
	if traceWriter != nil {
		cleanup = func() {
			if err = traceWriter.Close(); err != nil {
				svrCtx.Logger.Error("failed to close trace writer", "err", err)
			}
		}
	}

	return traceWriter, cleanup, nil
}

// StartGrpcServer starts a gRPC server with the provided configuration.
// It returns the gRPC server instance, updated client context with historical connections,
// and any error encountered during setup.
//
// The function will:
// - Create a gRPC client connection
// - Setup historical gRPC connections if configured
// - Start the gRPC server in a goroutine
//
// Note: The provided context will ensure that the server is gracefully shut down.
func StartGrpcServer(
	ctx context.Context,
	g *errgroup.Group,
	config serverconfig.GRPCConfig,
	clientCtx client.Context,
	svrCtx *Context,
	app types.Application,
) (*grpc.Server, client.Context, error) {
	if !config.Enable {
		// return grpcServer as nil if gRPC is disabled
		return nil, clientCtx, nil
	}
	_, _, err := net.SplitHostPort(config.Address)
	if err != nil {
		return nil, clientCtx, err
	}

	maxSendMsgSize := config.MaxSendMsgSize
	if maxSendMsgSize == 0 {
		maxSendMsgSize = serverconfig.DefaultGRPCMaxSendMsgSize
	}

	maxRecvMsgSize := config.MaxRecvMsgSize
	if maxRecvMsgSize == 0 {
		maxRecvMsgSize = serverconfig.DefaultGRPCMaxRecvMsgSize
	}

	// if gRPC is enabled, configure gRPC client for gRPC gateway
	grpcClient, err := grpc.NewClient(
		config.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.ForceCodec(codec.NewProtoCodec(clientCtx.InterfaceRegistry).GRPCCodec()),
			grpc.MaxCallRecvMsgSize(maxRecvMsgSize),
			grpc.MaxCallSendMsgSize(maxSendMsgSize),
		),
	)
	if err != nil {
		return nil, clientCtx, err
	}

	clientCtx = clientCtx.WithGRPCClient(grpcClient)
	svrCtx.Logger.Debug("gRPC client assigned to client context", "target", config.Address)

	logger := svrCtx.Logger.With("module", "grpc-server")
	var grpcSrv *grpc.Server
	grpcSrv, clientCtx, err = servergrpc.NewGRPCServerAndContext(clientCtx, app, config, logger)
	if err != nil {
		return nil, clientCtx, err
	}

	// Start the gRPC server in a goroutine. Note, the provided ctx will ensure
	// that the server is gracefully shut down.
	g.Go(func() error {
		return servergrpc.StartGRPCServer(ctx, logger, config, grpcSrv)
	})
	return grpcSrv, clientCtx, nil
}

func startAPIServer(
	ctx context.Context,
	g *errgroup.Group,
	svrCfg serverconfig.Config,
	clientCtx client.Context,
	svrCtx *Context,
	app types.Application,
	home string,
	grpcSrv *grpc.Server,
	metrics *telemetry.Metrics,
) error {
	if !svrCfg.API.Enable {
		return nil
	}

	clientCtx = clientCtx.WithHomeDir(home)

	apiSrv := api.New(clientCtx, svrCtx.Logger.With("module", "api-server"), grpcSrv)
	app.RegisterAPIRoutes(apiSrv, svrCfg.API)

	if svrCfg.Telemetry.Enabled {
		apiSrv.SetTelemetry(metrics)
	}

	g.Go(func() error {
		return apiSrv.Start(ctx, svrCfg)
	})
	return nil
}

func startTelemetry(cfg serverconfig.Config) (*telemetry.Metrics, error) {
	return telemetry.New(cfg.Telemetry)
}

// wrapCPUProfile starts CPU profiling, if enabled, and executes the provided
// callbackFn in a separate goroutine, then will wait for that callback to
// return.
//
// NOTE: We expect the caller to handle graceful shutdown and signal handling.
func wrapCPUProfile(svrCtx *Context, callbackFn func() error) error {
	if cpuProfile := svrCtx.Viper.GetString(flagCPUProfile); cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			return err
		}

		svrCtx.Logger.Info("starting CPU profiler", "profile", cpuProfile)

		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}

		defer func() {
			svrCtx.Logger.Info("stopping CPU profiler", "profile", cpuProfile)
			pprof.StopCPUProfile()

			if err := f.Close(); err != nil {
				svrCtx.Logger.Info("failed to close cpu-profile file", "profile", cpuProfile, "err", err.Error())
			}
		}()
	}

	return callbackFn()
}

// emitServerInfoMetrics emits server info related metrics using application telemetry.
func emitServerInfoMetrics() {
	var ls []metrics.Label

	versionInfo := version.NewInfo()
	if len(versionInfo.GoVersion) > 0 {
		ls = append(ls, telemetry.NewLabel("go", versionInfo.GoVersion))
	}
	if len(versionInfo.CosmosSdkVersion) > 0 {
		ls = append(ls, telemetry.NewLabel("version", versionInfo.CosmosSdkVersion))
	}

	if len(ls) == 0 {
		return
	}

	telemetry.SetGaugeWithLabels([]string{"server", "info"}, 1, ls)
}

func getCtx(svrCtx *Context, block bool) (*errgroup.Group, context.Context) {
	ctx, cancelFn := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	// listen for quit signals so the calling parent process can gracefully exit
	ListenForQuitSignals(g, block, cancelFn, svrCtx.Logger)
	return g, ctx
}

func startApp(svrCtx *Context, appCreator types.AppCreator, opts StartCmdOptions) (app types.Application, cleanupFn func(), err error) {
	traceWriter, traceCleanupFn, err := setupTraceWriter(svrCtx)
	if err != nil {
		return app, traceCleanupFn, err
	}

	home := svrCtx.Config.RootDir
	db, err := opts.DBOpener(home, GetAppDBBackend(svrCtx.Viper))
	if err != nil {
		return app, traceCleanupFn, err
	}

	if isTestnet, ok := svrCtx.Viper.Get(KeyIsTestnet).(bool); ok && isTestnet {
		app, err = testnetify(svrCtx, appCreator, db, traceWriter)
		if err != nil {
			return app, traceCleanupFn, err
		}
	} else {
		app = appCreator(svrCtx.Logger, db, traceWriter, svrCtx.Viper)
	}

	cleanupFn = func() {
		traceCleanupFn()
		if localErr := app.Close(); localErr != nil {
			svrCtx.Logger.Error(localErr.Error())
		}
	}
	return app, cleanupFn, nil
}

// InPlaceTestnetCreator utilizes the provided chainID and operatorAddress as well as the local private validator key to
// control the network represented in the data folder. This is useful to create testnets nearly identical to your
// mainnet environment.
func InPlaceTestnetCreator(testnetAppCreator types.AppCreator, extraOpts ...StartCmdOptions) *cobra.Command {
	opts := StartCmdOptions{}
	if len(extraOpts) > 0 {
		opts = extraOpts[0]
	}
	if opts.DBOpener == nil {
		opts.DBOpener = openDB
	}

	if opts.StartCommandHandler == nil {
		opts.StartCommandHandler = start
	}

	cmd := &cobra.Command{
		Use:   "in-place-testnet",
		Short: "Create and start a testnet from current local state",
		Long: `Create and start a testnet from current local state.
After utilizing this command the network will start. If the network is stopped,
the normal "start" command should be used. Re-using this command on state that
has already been modified by this command could result in unexpected behavior.

Additionally, the first block may take up to one minute to be committed, depending
on how old the block is. For instance, if a snapshot was taken weeks ago and we want
to turn this into a testnet, it is possible lots of pending state needs to be committed
(expiring locks, etc.). It is recommended that you should wait for this block to be committed
before stopping the daemon.

If the --trigger-testnet-upgrade flag is set, the upgrade handler specified by the flag will be run
on the first block of the testnet.

Regardless of whether the flag is set or not, if any new stores are introduced in the daemon being run,
those stores will be registered in order to prevent panics. Therefore, you only need to set the flag if
you want to test the upgrade handler itself.
`,
		Example: "in-place-testnet --new-chain-ID stabletestnet_2201-1",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			serverCtx := GetServerContextFromCmd(cmd)
			_, err := GetPruningOptionsFromFlags(serverCtx.Viper)
			if err != nil {
				return err
			}

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			withCMT, _ := cmd.Flags().GetBool(flagWithComet)
			if !withCMT {
				serverCtx.Logger.Info("starting ABCI without CometBFT")
			}

			newChainID := serverCtx.Viper.GetString(KeyNewChainID)
			if newChainID == "" {
				return fmt.Errorf("--%s is required", KeyNewChainID)
			}

			skipConfirmation, _ := cmd.Flags().GetBool("skip-confirmation")

			if !skipConfirmation {
				// Confirmation prompt to prevent accidental modification of state.
				reader := bufio.NewReader(os.Stdin)
				fmt.Println("This operation will modify state in your data folder and cannot be undone. Do you want to continue? (y/n)")
				text, _ := reader.ReadString('\n')
				response := strings.TrimSpace(strings.ToLower(text))
				if response != "y" && response != "yes" {
					fmt.Println("Operation canceled.")
					return nil
				}
			}

			// Set testnet keys to be used by the application.
			// This is done to prevent changes to existing start API.
			serverCtx.Viper.Set(KeyIsTestnet, true)
			serverCtx.Viper.Set(KeyNewChainID, newChainID)
			numValidators, _ := cmd.Flags().GetInt(KeyTestnetValidators)
			if numValidators < 1 {
				numValidators = 1
			}
			serverCtx.Viper.Set(KeyTestnetValidators, numValidators)

			outputDir, _ := cmd.Flags().GetString(KeyTestnetOutputDir)
			serverCtx.Viper.Set(KeyTestnetOutputDir, outputDir)

			if numValidators > 1 {
				// Multi-validator workflow:
				// 1. testnetify with single validator (100% power) so one node can produce blocks
				// 2. Start node with halt-height = lastBlockHeight+1 to run upgrade and commit one block
				// 3. After halt, re-open state and swap validator set to multi-validator
				// 4. Export node directories

				// NOTE: Do NOT override KeyTestnetValidators here.
				// CometBFT's testnetify always uses single validator (hardcoded),
				// but the app layer (InitStableForTestnet) needs the real numValidators
				// to register all validators in the staking module during the first block.

				home := serverCtx.Config.RootDir
				lastHeight, err := getLastBlockHeight(home)
				if err != nil {
					return fmt.Errorf("failed to determine last block height: %w", err)
				}

				// Phase 1: testnetify (single val) + start node + halt after first block.
				// The halt-height mechanism causes a CometBFT "CONSENSUS FAILURE" panic
				// which is expected — the first block has already been committed by that point.
				// We send SIGINT to trigger a clean shutdown after a short delay.
				serverCtx.Logger.Info("Phase 1: initializing single-validator testnet and committing first block...")

				// Poll the local RPC endpoint to detect first block commit, then SIGINT.
				rpcAddr := serverCtx.Config.RPC.ListenAddress
				go func() {
					// Wait for RPC to become available, then poll /status for height.
					ticker := time.NewTicker(1 * time.Second)
					defer ticker.Stop()
					for range ticker.C {
						resp, err := http.Get(fmt.Sprintf("http://%s/status", strings.TrimPrefix(rpcAddr, "tcp://")))
						if err != nil {
							continue
						}
						var result struct {
							Result struct {
								SyncInfo struct {
									LatestBlockHeight string `json:"latest_block_height"`
								} `json:"sync_info"`
							} `json:"result"`
						}
						if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
							resp.Body.Close()
							continue
						}
						resp.Body.Close()
						h, _ := strconv.ParseInt(result.Result.SyncInfo.LatestBlockHeight, 10, 64)
						if h > lastHeight {
							serverCtx.Logger.Info("First block committed, sending shutdown signal...", "height", h)
							p, _ := os.FindProcess(os.Getpid())
							p.Signal(os.Interrupt)
							return
						}
					}
				}()
				_ = wrapCPUProfile(serverCtx, func() error {
					return opts.StartCommandHandler(serverCtx, clientCtx, testnetAppCreator, withCMT, opts)
				})

				// Phase 2: swap validator set to multi-validator and export.
				serverCtx.Logger.Info("Phase 2: converting to multi-validator and exporting node directories...",
					"validators", numValidators, "output", outputDir)
				if err := convertToMultiValidator(serverCtx, numValidators, outputDir, opts); err != nil {
					return fmt.Errorf("failed to convert to multi-validator: %w", err)
				}
				fmt.Printf("Successfully exported %d validator node directories to %s\n", numValidators, outputDir)
				fmt.Println("Start each node separately with: stabled start --home <node-dir>")
				return nil
			}

			err = wrapCPUProfile(serverCtx, func() error {
				return opts.StartCommandHandler(serverCtx, clientCtx, testnetAppCreator, withCMT, opts)
			})

			serverCtx.Logger.Debug("received quit signal")
			graceDuration, _ := cmd.Flags().GetDuration(FlagShutdownGrace)
			if graceDuration > 0 {
				serverCtx.Logger.Info("graceful shutdown start", FlagShutdownGrace, graceDuration)
				<-time.After(graceDuration)
				serverCtx.Logger.Info("graceful shutdown complete")
			}

			return err
		},
	}

	addStartNodeFlags(cmd, opts)
	cmd.Flags().String(KeyNewChainID, "", "New chain ID for the testnet (required)")
	cmd.Flags().String(KeyTriggerTestnetUpgrade, "", "If set (example: \"v21\"), triggers the v21 upgrade handler to run on the first block of the testnet")
	cmd.Flags().Bool("skip-confirmation", false, "Skip the confirmation prompt")
	cmd.Flags().Int(KeyTestnetValidators, 1, "Total number of validators to create (1 = only the primary signing validator)")
	cmd.Flags().String(KeyTestnetOutputDir, "out", "Output directory for exported validator node directories")
	return cmd
}

// testnetify modifies both state and blockStore, allowing the provided operator address and local validator key to control the network
// that the state in the data folder represents. The chainID of the local genesis file is modified to match the provided chainID.
func testnetify(ctx *Context, testnetAppCreator types.AppCreator, db dbm.DB, traceWriter io.WriteCloser) (types.Application, error) {
	config := ctx.Config

	newChainID, ok := ctx.Viper.Get(KeyNewChainID).(string)
	if !ok {
		return nil, fmt.Errorf("expected string for key %s", KeyNewChainID)
	}

	// Modify app genesis chain ID and save to genesis file.
	genFilePath := config.GenesisFile()
	appGen, err := genutiltypes.AppGenesisFromFile(genFilePath)
	if err != nil {
		return nil, err
	}
	appGen.ChainID = newChainID
	if err := appGen.ValidateAndComplete(); err != nil {
		return nil, err
	}
	if err := appGen.SaveAs(genFilePath); err != nil {
		return nil, err
	}

	// Regenerate addrbook.json to prevent peers on old network from causing error logs.
	addrBookPath := filepath.Join(config.RootDir, "config", "addrbook.json")
	if err := os.Remove(addrBookPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove existing addrbook.json: %w", err)
	}

	emptyAddrBook := []byte("{}")
	if err := os.WriteFile(addrBookPath, emptyAddrBook, 0o600); err != nil {
		return nil, fmt.Errorf("failed to create empty addrbook.json: %w", err)
	}

	// Load the comet genesis doc provider.
	genDocProvider := node.DefaultGenesisDocProviderFunc(config)

	// Initialize blockStore and stateDB.
	blockStoreDB, err := cmtcfg.DefaultDBProvider(&cmtcfg.DBContext{ID: "blockstore", Config: config})
	if err != nil {
		return nil, err
	}
	blockStore := store.NewBlockStore(blockStoreDB)

	stateDB, err := cmtcfg.DefaultDBProvider(&cmtcfg.DBContext{ID: "state", Config: config})
	if err != nil {
		return nil, err
	}

	defer blockStore.Close()
	defer stateDB.Close()

	privValidator := pvm.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	userPubKey, err := privValidator.GetPubKey()
	if err != nil {
		return nil, err
	}
	validatorAddress := userPubKey.Address()

	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: config.Storage.DiscardABCIResponses,
	})

	state, genDoc, err := node.LoadStateFromDBOrGenesisDocProvider(stateDB, genDocProvider)
	if err != nil {
		return nil, err
	}

	ctx.Viper.Set(KeyNewValAddr, validatorAddress)
	ctx.Viper.Set(KeyUserPubKey, userPubKey)
	testnetApp := testnetAppCreator(ctx.Logger, db, traceWriter, ctx.Viper)

	// We need to create a temporary proxyApp to get the initial state of the application.
	// Depending on how the node was stopped, the application height can differ from the blockStore height.
	// This height difference changes how we go about modifying the state.
	cmtApp := NewCometABCIWrapper(testnetApp)
	_, context := getCtx(ctx, true)
	clientCreator := proxy.NewLocalClientCreator(cmtApp)
	metrics := node.DefaultMetricsProvider(cmtcfg.DefaultConfig().Instrumentation)
	_, _, _, _, proxyMetrics, _, _ := metrics(genDoc.ChainID)
	proxyApp := proxy.NewAppConns(clientCreator, proxyMetrics)
	if err := proxyApp.Start(); err != nil {
		return nil, fmt.Errorf("error starting proxy app connections: %w", err)
	}
	res, err := proxyApp.Query().Info(context, proxy.RequestInfo)
	if err != nil {
		return nil, fmt.Errorf("error calling Info: %w", err)
	}
	err = proxyApp.Stop()
	if err != nil {
		return nil, err
	}
	appHash := res.LastBlockAppHash
	appHeight := res.LastBlockHeight

	var block *cmttypes.Block
	switch {
	case appHeight == blockStore.Height():
		block = blockStore.LoadBlock(blockStore.Height())
		// If the state's last blockstore height does not match the app and blockstore height, we likely stopped with the halt height flag.
		if state.LastBlockHeight != appHeight {
			state.LastBlockHeight = appHeight
			block.AppHash = appHash
			state.AppHash = appHash
		} else {
			// Node was likely stopped via SIGTERM, delete the next block's seen commit
			err := blockStoreDB.Delete(fmt.Appendf(nil, "SC:%v", blockStore.Height()+1))
			if err != nil {
				return nil, err
			}
		}
	case blockStore.Height() > state.LastBlockHeight:
		// This state usually occurs when we gracefully stop the node.
		err = blockStore.DeleteLatestBlock()
		if err != nil {
			return nil, err
		}
		block = blockStore.LoadBlock(blockStore.Height())
	default:
		// If there is any other state, we just load the block
		block = blockStore.LoadBlock(blockStore.Height())
	}

	block.ChainID = newChainID
	state.ChainID = newChainID

	block.LastBlockID = state.LastBlockID
	block.LastCommit.BlockID = state.LastBlockID

	// Create a single-validator set with the local validator key.
	newVal := &cmttypes.Validator{
		Address:     validatorAddress,
		PubKey:      userPubKey,
		VotingPower: 900000000000000,
	}
	newValSet := &cmttypes.ValidatorSet{
		Validators: []*cmttypes.Validator{newVal},
		Proposer:   newVal,
	}

	// Build commit signature from our single validator.
	vote := cmttypes.Vote{
		Type:             cmtproto.PrecommitType,
		Height:           state.LastBlockHeight,
		Round:            0,
		BlockID:          state.LastBlockID,
		Timestamp:        time.Now(),
		ValidatorAddress: validatorAddress,
		ValidatorIndex:   0,
		Signature:        []byte{},
	}
	voteProto := vote.ToProto()
	err = privValidator.SignVote(newChainID, voteProto)
	if err != nil {
		return nil, err
	}
	vote.Signature = voteProto.Signature
	vote.Timestamp = voteProto.Timestamp

	// Overwrite block's lastCommit and seenCommit.
	block.LastCommit.Signatures = []cmttypes.CommitSig{{
		BlockIDFlag:      cmttypes.BlockIDFlagCommit,
		ValidatorAddress: validatorAddress,
		Timestamp:        vote.Timestamp,
		Signature:        vote.Signature,
	}}
	block.LastCommit.BlockID = state.LastBlockID
	block.LastCommit.Round = 0

	seenCommit := blockStore.LoadSeenCommit(state.LastBlockHeight)
	seenCommit.BlockID = state.LastBlockID
	seenCommit.Round = 0
	seenCommit.Signatures = block.LastCommit.Signatures
	err = blockStore.SaveSeenCommit(state.LastBlockHeight, seenCommit)
	if err != nil {
		return nil, err
	}

	// Replace all valSets in state.
	state.Validators = newValSet
	state.LastValidators = newValSet
	state.NextValidators = newValSet
	state.LastHeightValidatorsChanged = blockStore.Height()

	err = stateStore.Save(state)
	if err != nil {
		return nil, err
	}

	// Create a ValidatorsInfo struct to store in stateDB.
	valSet, err := state.Validators.ToProto()
	if err != nil {
		return nil, err
	}
	valInfo := &cmtstate.ValidatorsInfo{
		ValidatorSet:      valSet,
		LastHeightChanged: state.LastBlockHeight,
	}
	buf, err := valInfo.Marshal()
	if err != nil {
		return nil, err
	}

	// Modfiy Validators stateDB entry.
	err = stateDB.Set(fmt.Appendf(nil, "validatorsKey:%v", blockStore.Height()), buf)
	if err != nil {
		return nil, err
	}

	// Modify LastValidators stateDB entry.
	err = stateDB.Set(fmt.Appendf(nil, "validatorsKey:%v", blockStore.Height()-1), buf)
	if err != nil {
		return nil, err
	}

	// Modify NextValidators stateDB entry.
	err = stateDB.Set(fmt.Appendf(nil, "validatorsKey:%v", blockStore.Height()+1), buf)
	if err != nil {
		return nil, err
	}

	// Since we modified the chainID, we set the new genesisDoc in the stateDB.
	b, err := cmtjson.Marshal(genDoc)
	if err != nil {
		return nil, err
	}
	if err := stateDB.SetSync([]byte("genesisDoc"), b); err != nil {
		return nil, err
	}

	return testnetApp, err
}

// deriveTestnetValidatorKey deterministically derives an ed25519 private key
// for a testnet validator at the given index. The app layer must use the exact
// same seed formula to produce matching staking-module validators.
func deriveTestnetValidatorKey(index int) cmted25519.PrivKey {
	secret := []byte(fmt.Sprintf("stable-testnet-validator-%d", index))
	return cmted25519.GenPrivKeyFromSecret(secret)
}

// getLastBlockHeight opens the block store to read the latest block height
// without starting the full application.
func getLastBlockHeight(home string) (int64, error) {
	cfg := cmtcfg.DefaultConfig()
	cfg.SetRoot(home)

	blockStoreDB, err := cmtcfg.DefaultDBProvider(&cmtcfg.DBContext{ID: "blockstore", Config: cfg})
	if err != nil {
		return 0, err
	}
	defer blockStoreDB.Close()

	blockStore := store.NewBlockStore(blockStoreDB)
	defer blockStore.Close()

	return blockStore.Height(), nil
}

// convertToMultiValidator re-opens the state/block stores after the single-validator
// node has halted, replaces the validator set with N validators (primary + derived),
// re-signs the last commit, and exports node directories.
func convertToMultiValidator(ctx *Context, numValidators int, outputDir string, opts StartCmdOptions) error {
	config := ctx.Config

	newChainID, ok := ctx.Viper.Get(KeyNewChainID).(string)
	if !ok {
		return fmt.Errorf("expected string for key %s", KeyNewChainID)
	}

	// Open block store and state store.
	blockStoreDB, err := cmtcfg.DefaultDBProvider(&cmtcfg.DBContext{ID: "blockstore", Config: config})
	if err != nil {
		return err
	}
	defer blockStoreDB.Close()
	blockStore := store.NewBlockStore(blockStoreDB)
	defer blockStore.Close()

	stateDB, err := cmtcfg.DefaultDBProvider(&cmtcfg.DBContext{ID: "state", Config: config})
	if err != nil {
		return err
	}
	defer stateDB.Close()

	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: config.Storage.DiscardABCIResponses,
	})

	genDocProvider := node.DefaultGenesisDocProviderFunc(config)
	state, _, err := node.LoadStateFromDBOrGenesisDocProvider(stateDB, genDocProvider)
	if err != nil {
		return err
	}

	// Load primary validator raw key (bypass FilePV.SignVote to avoid double-sign protection
	// which rejects re-signing at the same height after Phase 1 committed a block).
	privValidator := pvm.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	primaryPrivKey := privValidator.Key.PrivKey
	userPubKey := primaryPrivKey.PubKey()
	validatorAddress := userPubKey.Address()

	// Build multi-validator set.
	totalPower := int64(900000000000000)
	perValPower := totalPower / int64(numValidators)

	// Map validator address → private key for signing.
	privKeys := make(map[string]crypto.PrivKey, numValidators)
	privKeys[validatorAddress.String()] = primaryPrivKey

	rawVals := make([]*cmttypes.Validator, numValidators)
	rawVals[0] = &cmttypes.Validator{
		Address:     validatorAddress,
		PubKey:      userPubKey,
		VotingPower: perValPower,
	}
	for i := 1; i < numValidators; i++ {
		pk := deriveTestnetValidatorKey(i)
		pub := pk.PubKey()
		privKeys[pub.Address().String()] = pk
		rawVals[i] = &cmttypes.Validator{
			Address:     pub.Address(),
			PubKey:      pub,
			VotingPower: perValPower,
		}
	}

	newValSet := cmttypes.NewValidatorSet(rawVals)

	// Build commit signatures in sorted validator order using raw keys.
	now := time.Now()
	sigs := make([]cmttypes.CommitSig, newValSet.Size())
	for idx, val := range newValSet.Validators {
		pk := privKeys[val.Address.String()]
		v := &cmtproto.Vote{
			Type:             cmtproto.PrecommitType,
			Height:           state.LastBlockHeight,
			Round:            0,
			BlockID:          state.LastBlockID.ToProto(),
			Timestamp:        now,
			ValidatorAddress: val.Address,
			ValidatorIndex:   int32(idx),
		}
		signBytes := cmttypes.VoteSignBytes(newChainID, v)
		sig, signErr := pk.Sign(signBytes)
		if signErr != nil {
			return fmt.Errorf("error signing vote for validator %X: %w", val.Address, signErr)
		}
		sigs[idx] = cmttypes.CommitSig{
			BlockIDFlag:      cmttypes.BlockIDFlagCommit,
			ValidatorAddress: val.Address,
			Timestamp:        now,
			Signature:        sig,
		}
	}

	// Update only the seenCommit (commit that finalized the latest block).
	// Do NOT modify block.LastCommit — it commits the previous block and must
	// remain valid against the previous validator set (single validator from Phase 1).
	seenCommit := blockStore.LoadSeenCommit(state.LastBlockHeight)
	seenCommit.BlockID = state.LastBlockID
	seenCommit.Round = 0
	seenCommit.Signatures = sigs
	if err := blockStore.SaveSeenCommit(state.LastBlockHeight, seenCommit); err != nil {
		return err
	}

	// Replace validator sets in state.
	state.Validators = newValSet
	state.LastValidators = newValSet
	state.NextValidators = newValSet
	state.LastHeightValidatorsChanged = blockStore.Height()

	if err := stateStore.Save(state); err != nil {
		return err
	}

	// Update stateDB validator entries.
	valSet, err := state.Validators.ToProto()
	if err != nil {
		return err
	}
	valInfo := &cmtstate.ValidatorsInfo{
		ValidatorSet:      valSet,
		LastHeightChanged: state.LastBlockHeight,
	}
	buf, err := valInfo.Marshal()
	if err != nil {
		return err
	}
	for _, h := range []int64{blockStore.Height() - 1, blockStore.Height(), blockStore.Height() + 1} {
		if err := stateDB.Set(fmt.Appendf(nil, "validatorsKey:%v", h), buf); err != nil {
			return err
		}
	}

	// Close stores before copying for export.
	blockStore.Close()
	blockStoreDB.Close()
	stateDB.Close()

	// Export node directories.
	return exportValidatorNodeDirs(config, ctx.Viper, outputDir, numValidators, opts)
}

// exportValidatorNodeDirs creates node{idx}/ directories under outputDir,
// each containing a copy of the primary node's data directory, a unique
// priv_validator_key.json, a unique node_key.json, and pre-configured
// peering so that all nodes can discover each other on localhost.
//
// Port layout per node (base ports offset by index):
//
//	P2P:  26656 + i*100
//	RPC:  26657 + i*100
//	ABCI: 26658 + i*100
//	gRPC: 9090  + i
//	API:  1317  + i
func exportValidatorNodeDirs(config *cmtcfg.Config, v *viper.Viper, outputDir string, numValidators int, opts StartCmdOptions) error {
	srcHome := config.RootDir

	// Determine testnet vs mainnet from upgrade flag.
	upgradeFlag := v.GetString(KeyTriggerTestnetUpgrade)
	isTestnet := strings.Contains(strings.ToLower(upgradeFlag), "rc")

	// First pass: copy dirs, write keys, generate node IDs.
	nodeIDs := make([]string, numValidators)
	for i := 0; i < numValidators; i++ {
		nodeDir := filepath.Join(outputDir, fmt.Sprintf("node%d", i))

		if err := copyDir(srcHome, nodeDir); err != nil {
			return fmt.Errorf("copy node%d: %w", i, err)
		}

		// Overwrite priv_validator_key for additional validators.
		if i > 0 {
			pk := deriveTestnetValidatorKey(i)
			keyFilePath := filepath.Join(nodeDir, "config", "priv_validator_key.json")
			stateFilePath := filepath.Join(nodeDir, "data", "priv_validator_state.json")
			filePV := pvm.NewFilePV(pk, keyFilePath, stateFilePath)
			filePV.Save()
		}

		// Generate a unique node_key.json for each node so they have distinct p2p identities.
		nodeKeyFile := filepath.Join(nodeDir, "config", "node_key.json")
		_ = os.Remove(nodeKeyFile) // remove copied key to force regeneration
		nodeKey, err := p2p.LoadOrGenNodeKey(nodeKeyFile)
		if err != nil {
			return fmt.Errorf("generate node key for node%d: %w", i, err)
		}
		nodeIDs[i] = string(nodeKey.ID())
	}

	// Second pass: write config.toml and modify app.toml in-place with peering and port settings.
	for i := 0; i < numValidators; i++ {
		nodeDir := filepath.Join(outputDir, fmt.Sprintf("node%d", i))

		var peers []string
		for j := 0; j < numValidators; j++ {
			if j == i {
				continue
			}
			peers = append(peers, fmt.Sprintf("%s@127.0.0.1:%d", nodeIDs[j], 26656+j*100))
		}

		// Clone the source CometBFT config and adjust per-node settings.
		nodeCfg := *config
		nodeCfg.SetRoot(nodeDir)
		nodeCfg.Moniker = fmt.Sprintf("node%d", i)
		nodeCfg.P2P.ListenAddress = fmt.Sprintf("tcp://0.0.0.0:%d", 26656+i*100)
		nodeCfg.P2P.PersistentPeers = strings.Join(peers, ",")
		nodeCfg.P2P.AddrBookStrict = false
		nodeCfg.P2P.AllowDuplicateIP = true
		nodeCfg.RPC.ListenAddress = fmt.Sprintf("tcp://0.0.0.0:%d", 26657+i*100)

		cmtcfg.WriteConfigFile(filepath.Join(nodeDir, "config", "config.toml"), &nodeCfg)

		// Modify the copied app.toml in-place using Viper.
		// This preserves all app-specific sections (EVM, JSON-RPC, etc.) that
		// serverconfig.WriteConfigFile would strip.
		nodeViper := viper.New()
		nodeViper.SetConfigFile(filepath.Join(nodeDir, "config", "app.toml"))
		if err := nodeViper.ReadInConfig(); err != nil {
			return fmt.Errorf("read app.toml for node%d: %w", i, err)
		}

		// Adjust SDK ports per node.
		nodeViper.Set("api.address", fmt.Sprintf("tcp://0.0.0.0:%d", 1317+i))
		nodeViper.Set("grpc.address", fmt.Sprintf("0.0.0.0:%d", 9090+i))

		// Adjust JSON-RPC ports per node (if the section exists in the copied config).
		if nodeViper.IsSet("json-rpc.address") {
			nodeViper.Set("json-rpc.address", fmt.Sprintf("127.0.0.1:%d", 8545+i))
			nodeViper.Set("json-rpc.ws-address", fmt.Sprintf("127.0.0.1:%d", 8546+i*100))

			// Enable JSON-RPC on node0 with broader access for development.
			if i == 0 {
				nodeViper.Set("json-rpc.enable", true)
				nodeViper.Set("json-rpc.address", "0.0.0.0:8545")
				nodeViper.Set("json-rpc.api", "eth,web3,debug,net")
			}
		}

		// Allow the app layer to apply testnet/mainnet-specific config.
		if opts.TestnetConfigModifier != nil {
			opts.TestnetConfigModifier(nodeViper, isTestnet, i)
		}

		if err := nodeViper.WriteConfig(); err != nil {
			return fmt.Errorf("write app.toml for node%d: %w", i, err)
		}
	}

	return nil
}

// copyDir recursively copies src directory to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// addStartNodeFlags should be added to any CLI commands that start the network.
func addStartNodeFlags(cmd *cobra.Command, opts StartCmdOptions) {
	cmd.Flags().Bool(flagWithComet, true, "Run abci app embedded in-process with CometBFT")
	cmd.Flags().String(flagAddress, "tcp://127.0.0.1:26658", "Listen address")
	cmd.Flags().String(flagTransport, "socket", "Transport protocol: socket, grpc")
	cmd.Flags().String(flagTraceStore, "", "Enable KVStore tracing to an output file")
	cmd.Flags().String(FlagMinGasPrices, "", "Minimum gas prices to accept for transactions; Any fee in a tx must meet this minimum (e.g. 0.01photino;0.0001stake)")
	cmd.Flags().Uint64(FlagQueryGasLimit, 0, "Maximum gas a Rest/Grpc query can consume. Blank and 0 imply unbounded.")
	cmd.Flags().IntSlice(FlagUnsafeSkipUpgrades, []int{}, "Skip a set of upgrade heights to continue the old binary")
	cmd.Flags().Uint64(FlagHaltHeight, 0, "Block height at which to gracefully halt the chain and shutdown the node")
	cmd.Flags().Uint64(FlagHaltTime, 0, "Minimum block time (in Unix seconds) at which to gracefully halt the chain and shutdown the node")
	cmd.Flags().Bool(FlagInterBlockCache, true, "Enable inter-block caching")
	cmd.Flags().String(flagCPUProfile, "", "Enable CPU profiling and write to the provided file")
	cmd.Flags().Bool(FlagTrace, false, "Provide full stack traces for errors in ABCI Log")
	cmd.Flags().String(FlagPruning, pruningtypes.PruningOptionDefault, "Pruning strategy (default|nothing|everything|custom)")
	cmd.Flags().Uint64(FlagPruningKeepRecent, 0, "Number of recent heights to keep on disk (ignored if pruning is not 'custom')")
	cmd.Flags().Uint64(FlagPruningInterval, 0, "Height interval at which pruned heights are removed from disk (ignored if pruning is not 'custom')")
	cmd.Flags().Uint(FlagInvCheckPeriod, 0, "Assert registered invariants every N blocks")
	cmd.Flags().Uint64(FlagMinRetainBlocks, 0, "Minimum block height offset during ABCI commit to prune CometBFT blocks")
	cmd.Flags().Bool(FlagAPIEnable, false, "Define if the API server should be enabled")
	cmd.Flags().Bool(FlagAPISwagger, false, "Define if swagger documentation should automatically be registered (Note: the API must also be enabled)")
	cmd.Flags().String(FlagAPIAddress, serverconfig.DefaultAPIAddress, "the API server address to listen on")
	cmd.Flags().Uint(FlagAPIMaxOpenConnections, 1000, "Define the number of maximum open connections")
	cmd.Flags().Uint(FlagRPCReadTimeout, 10, "Define the CometBFT RPC read timeout (in seconds)")
	cmd.Flags().Uint(FlagRPCWriteTimeout, 0, "Define the CometBFT RPC write timeout (in seconds)")
	cmd.Flags().Uint(FlagRPCMaxBodyBytes, 1000000, "Define the CometBFT maximum request body (in bytes)")
	cmd.Flags().Bool(FlagAPIEnableUnsafeCORS, false, "Define if CORS should be enabled (unsafe - use it at your own risk)")
	cmd.Flags().Bool(flagGRPCOnly, false, "Start the node in gRPC query only mode (no CometBFT process is started)")
	cmd.Flags().Bool(flagGRPCEnable, true, "Define if the gRPC server should be enabled")
	cmd.Flags().String(flagGRPCAddress, serverconfig.DefaultGRPCAddress, "the gRPC server address to listen on")
	cmd.Flags().Bool(flagGRPCWebEnable, true, "Define if the gRPC-Web server should be enabled. (Note: gRPC must also be enabled)")
	cmd.Flags().String(flagHistoricalGRPCAddressBlockRange, "", "Define if historical grpc and block range is available")
	cmd.Flags().Uint64(FlagStateSyncSnapshotInterval, 0, "State sync snapshot interval")
	cmd.Flags().Uint32(FlagStateSyncSnapshotKeepRecent, 2, "State sync snapshot to keep")
	cmd.Flags().Bool(FlagDisableIAVLFastNode, false, "Disable fast node for IAVL tree")
	cmd.Flags().Int(FlagMempoolMaxTxs, mempool.DefaultMaxTx, "Sets MaxTx value for the app-side mempool")
	cmd.Flags().Duration(FlagShutdownGrace, 0*time.Second, "On Shutdown, duration to wait for resource clean up")

	// support old flags name for backwards compatibility
	cmd.Flags().SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "with-tendermint" {
			name = flagWithComet
		}

		return pflag.NormalizedName(name)
	})

	// add support for all CometBFT-specific command line options
	cmtcmd.AddNodeFlags(cmd)

	if opts.AddFlags != nil {
		opts.AddFlags(cmd)
	}
}
