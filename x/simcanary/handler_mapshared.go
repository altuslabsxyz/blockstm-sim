package simcanary

import (
	"context"

	"github.com/altuslabsxyz/blockstm-sim/x/simcanary/keeper"
	"github.com/altuslabsxyz/blockstm-sim/x/simcanary/types"
)

type msgServer struct {
	keeper *keeper.Keeper
}

func NewMsgServer(k *keeper.Keeper) types.MsgServer {
	return &msgServer{keeper: k}
}

func (s *msgServer) MapSet(_ context.Context, msg *types.MsgCanaryMapSet) (*types.MsgCanaryMapSetResponse, error) {
	s.keeper.SetMapValue(msg.Key, msg.Value)
	return &types.MsgCanaryMapSetResponse{}, nil
}

func (s *msgServer) MapReadAndWrite(ctx context.Context, msg *types.MsgCanaryMapReadAndWrite) (*types.MsgCanaryMapReadAndWriteResponse, error) {
	val := s.keeper.GetMapValue(msg.Key)
	if err := s.keeper.WriteToStore(ctx, msg.Key, val); err != nil {
		return nil, err
	}
	return &types.MsgCanaryMapReadAndWriteResponse{ObservedValue: val}, nil
}
