package types

import "github.com/cosmos/gogoproto/proto"

func init() {
	proto.RegisterType((*GenesisState)(nil), "ageverify.ageverify.GenesisState")
}

// GenesisState holds the initial state for the ageverify module.
// Empty — we start with no verified addresses.
type GenesisState struct{}

func (m *GenesisState) Reset()         { *m = GenesisState{} }
func (m *GenesisState) String() string  { return proto.CompactTextString(m) }
func (*GenesisState) ProtoMessage()     {}

func (g *GenesisState) ProtoSize() int { return 0 }

func NewGenesisState() *GenesisState { return &GenesisState{} }
