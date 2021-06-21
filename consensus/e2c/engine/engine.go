package engine

import (
	"bytes"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

func (e2c *E2C) Verify(header *types.Header) error {
	chain := e2c.broadcaster.ChainHeaderReader()
	if header.Number == nil {
		return errUnknownBlock
	}
	number := header.Number.Uint64()

	// Don't waste time checking blocks from the future (adjusting for allowed threshold)
	adjustedTimeNow := time.Now().Add(time.Duration(e2c.config.AllowedFutureBlockTime) * time.Second).Unix()
	if header.Time > uint64(adjustedTimeNow) {
		return consensus.ErrFutureBlock
	}
	// Check that the extra-data contains both the vanity and signature
	if len(header.Extra) < extraVanity {
		return errMissingVanity
	}
	if len(header.Extra) < extraVanity+extraSeal {
		return errMissingSignature
	}
	// TODO: Make this check when view changes rather than on checkpoint
	// Ensure that the extra-data contains a signer list on checkpoint, but none otherwise
	checkpoint := (number % e2c.config.Epoch) == 0
	signersBytes := len(header.Extra) - extraVanity - extraSeal
	if !checkpoint && signersBytes != 0 {
		return errExtraSigners
	}
	if checkpoint && signersBytes%common.AddressLength != 0 {
		return errInvalidCheckpointSigners
	}
	// Ensure that the mix digest is zero as we don't have fork protection currently
	if header.MixDigest != (common.Hash{}) {
		return errInvalidMixDigest
	}
	// Ensure that the block doesn't contain any uncles which are meaningless in PoA
	if header.UncleHash != uncleHash {
		return errInvalidUncleHash
	}
	// Ensure that the block's difficulty is correct
	if number > 0 {
		if header.Difficulty == nil {
			return errInvalidDifficulty
		}
		if header.Difficulty.Cmp(difficulty) != 0 {
			return errWrongDifficulty
		}
	}
	// If all checks passed, validate any special fields for hard forks
	if err := misc.VerifyForkHashes(chain.Config(), header, false); err != nil {
		return err
	}
	// The genesis block is the always valid dead-end
	if number == 0 {
		return nil
	}
	// Retrieve the snapshot needed to verify this header and cache it
	snap, err := e2c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return err
	}
	// TODO: Fix this when view change is added
	// If the block is a checkpoint block, verify the signer list
	if number%e2c.config.Epoch == 0 {
		signers := make([]byte, common.AddressLength)
		for i, signer := range snap.signers() {
			copy(signers[i*common.AddressLength:], signer[:])
		}
		extraSuffix := len(header.Extra) - extraSeal
		if !bytes.Equal(header.Extra[extraVanity:extraSuffix], signers) {
			return errMismatchingCheckpointSigners
		}
	}
	// Resolve the authorization key and check against signers
	signer, err := ecrecover(header, e2c.signatures)
	if err != nil {
		return err
	}
	if signer != snap.Signer {
		return errUnauthorizedSigner
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	return nil
}

func (e2c *E2C) EventMux() *event.TypeMux {
	return e2c.eventMux
}

func (e2c *E2C) Commit(block *types.Block) error {
	_, err := e2c.broadcaster.InsertBlock(block)
	return err
}

func (e2c *E2C) BroadcastBlock(block *types.Block) {
	go e2c.broadcaster.BroadcastBlock(block, false)
}

// TODO: Implement these
func (e2c *E2C) BroadcastBlame() {
}

func (e2c *E2C) RequestBlock(block *common.Hash) {
}
