package types

import sdk "github.com/cosmos/cosmos-sdk/types"

func (m *MsgSubmitAgeProof) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Creator)
	return err
}

func (m *MsgSubmitAgeProof) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{addr}
}
