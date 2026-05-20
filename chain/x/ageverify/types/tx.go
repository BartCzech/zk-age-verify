package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

func init() {
	proto.RegisterType((*MsgSubmitAgeProof)(nil), "ageverify.ageverify.MsgSubmitAgeProof")
	proto.RegisterType((*MsgSubmitAgeProofResponse)(nil), "ageverify.ageverify.MsgSubmitAgeProofResponse")
}

// MsgSubmitAgeProof is submitted by a user to prove they are >= 18
// without revealing their date of birth.
type MsgSubmitAgeProof struct {
	Creator       string `protobuf:"bytes,1,opt,name=creator,proto3"        json:"creator"`
	Proof         string `protobuf:"bytes,2,opt,name=proof,proto3"          json:"proof"`
	PublicWitness string `protobuf:"bytes,3,opt,name=public_witness,proto3" json:"public_witness"`
	CurrentDate   string `protobuf:"bytes,4,opt,name=current_date,proto3"   json:"current_date"`
}

func (m *MsgSubmitAgeProof) Reset()         { *m = MsgSubmitAgeProof{} }
func (m *MsgSubmitAgeProof) String() string  { return proto.CompactTextString(m) }
func (*MsgSubmitAgeProof) ProtoMessage()     {}

func (m *MsgSubmitAgeProof) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Creator)
	return err
}

func (m *MsgSubmitAgeProof) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{addr}
}

// MsgSubmitAgeProofResponse is empty — the event carries the result.
type MsgSubmitAgeProofResponse struct{}

func (m *MsgSubmitAgeProofResponse) Reset()         { *m = MsgSubmitAgeProofResponse{} }
func (m *MsgSubmitAgeProofResponse) String() string  { return proto.CompactTextString(m) }
func (*MsgSubmitAgeProofResponse) ProtoMessage()     {}
