package keeper

import (
	"context"
	"encoding/base64"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"ageverify/x/ageverify/types"
)

const (
	dateTimeTolerance = 24 * time.Hour
	// requiredMinAge is the age threshold enforced by the chain. It is NOT
	// taken from the transaction — a prover that supplied its own MinAge could
	// set it to 0 and pass trivially.
	requiredMinAge = 18
)

func (k msgServer) SubmitAgeProof(
	goCtx context.Context,
	msg *types.MsgSubmitAgeProof,
) (*types.MsgSubmitAgeProofResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// ---- Step 1: Decode proof from base64 ----
	proofBytes, err := base64.StdEncoding.DecodeString(msg.Proof)
	if err != nil {
		return nil, types.ErrInvalidProof.Wrap("base64 decode failed")
	}

	// ---- Step 2: Validate current_date vs block time ----
	// Prevents a prover from using a far-future date to fake age. The date is
	// also fed into the public witness below, so this bound is what stops the
	// prover from claiming an arbitrary "current" year.
	proofDate, err := time.Parse("20060102", msg.CurrentDate)
	if err != nil {
		return nil, types.ErrDateMismatch.Wrap("invalid format, expected YYYYMMDD")
	}
	blockTime := ctx.BlockTime()
	diff := blockTime.Sub(proofDate)
	if diff < 0 {
		diff = -diff
	}
	if diff > dateTimeTolerance {
		return nil, types.ErrDateMismatch.Wrap("proof date too far from block time")
	}

	// ---- Step 3: Build the public witness from TRUSTED values ----
	// The witness is reconstructed here from the chain-enforced MinAge and the
	// block-time-validated date — NOT from msg.PublicWitness. This is what
	// guarantees the proof actually attests "age >= 18 as of now", rather than
	// "age >= 0" or "age >= 18 as of year 2200". msg.PublicWitness is ignored.
	pubWitness, err := BuildPublicWitness(
		proofDate.Year(),
		int(proofDate.Month()),
		proofDate.Day(),
		requiredMinAge,
	)
	if err != nil {
		return nil, types.ErrInvalidWitness.Wrap(err.Error())
	}

	// ---- Step 4: Load verification key ----
	vk, err := LoadVerificationKey()
	if err != nil {
		return nil, types.ErrProofVerificationFailed.Wrap(err.Error())
	}

	// ---- Step 5: Verify ZK proof against the trusted public witness ----
	if err := VerifyAgeProof(vk, proofBytes, pubWitness); err != nil {
		return nil, types.ErrProofVerificationFailed.Wrap(err.Error())
	}

	// ---- Step 6: Store result ----
	k.Keeper.SetVerified(goCtx, msg.Creator, blockTime.Format(time.RFC3339))

	// ---- Step 7: Emit event ----
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"age_verified",
		sdk.NewAttribute("address", msg.Creator),
		sdk.NewAttribute("verified_at", blockTime.Format(time.RFC3339)),
	))

	return &types.MsgSubmitAgeProofResponse{}, nil
}
