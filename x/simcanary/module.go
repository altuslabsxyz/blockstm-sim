package simcanary

import (
	"cosmossdk.io/core/appmodule"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/depinject"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/altuslabsxyz/blockstm-sim/x/simcanary/keeper"
	"github.com/altuslabsxyz/blockstm-sim/x/simcanary/types"
)

var (
	_ appmodule.AppModule = AppModule{}
	_ module.HasServices  = AppModule{}
	_ module.HasName      = AppModule{}
)

type AppModule struct {
	keeper *keeper.Keeper
}

func NewAppModule(k *keeper.Keeper) AppModule {
	return AppModule{keeper: k}
}

func (AppModule) IsOnePerModuleType() {}
func (AppModule) IsAppModule()        {}

func (AppModule) Name() string            { return types.ModuleName }
func (AppModule) ConsensusVersion() uint64 { return 1 }

func (AppModule) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), NewMsgServer(am.keeper))
}

// ---------------------------------------------------------------------------
// Depinject wiring
// ---------------------------------------------------------------------------

func init() {
	appmodule.Register(&types.Module{},
		appmodule.Provide(ProvideModule),
	)
}

type ModuleInputs struct {
	depinject.In
	StoreService corestore.KVStoreService
}

type ModuleOutputs struct {
	depinject.Out
	Module appmodule.AppModule
	Keeper *keeper.Keeper
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	k := keeper.NewKeeper(in.StoreService)
	return ModuleOutputs{Module: NewAppModule(k), Keeper: k}
}
