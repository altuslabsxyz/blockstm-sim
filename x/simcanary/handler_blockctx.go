package simcanary

import (
	"context"

	"github.com/altuslabsxyz/blockstm-sim/x/simcanary/types"
)

func (s *msgServer) BlockContextSet(_ context.Context, msg *types.MsgCanaryBlockContextSet) (*types.MsgCanaryBlockContextSetResponse, error) {
	s.keeper.WriteBlockCtxField(msg.Field, msg.Value)
	return &types.MsgCanaryBlockContextSetResponse{}, nil
}

func (s *msgServer) BlockContextRead(_ context.Context, msg *types.MsgCanaryBlockContextRead) (*types.MsgCanaryBlockContextReadResponse, error) {
	val := s.keeper.ReadBlockCtxField(msg.Field)
	return &types.MsgCanaryBlockContextReadResponse{Value: val}, nil
}
