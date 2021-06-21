package engine

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/trie"
)

const (
	NewBlockMsg = 0x07
	AckMsg      = 0x0c
)

func (e *E2C) HandleMsg(p consensus.Peer, msg p2p.Msg) (bool, error) {
	if msg.Code == NewBlockMsg {
		var request struct {
			Block *types.Block
			TD    *big.Int
		}

		// All error checking, if any of these fail, don't handleMsg and let the caller deal with it
		// Most of these require some special handling that can't be done here due to due to circular
		// imports, so I just return this to caller so the caller can deal with. Not the most efficient
		// solution, but it should work for the moment. This is a section that could slightly improve
		// efficiency if needed later on
		if err := msg.Decode(&request); err != nil {
			return false, nil
		}
		if hash := types.DeriveSha(request.Block.Transactions(), new(trie.Trie)); hash != request.Block.TxHash() {
			return false, nil
		}
		if err := request.Block.SanityCheck(); err != nil {
			return false, nil
		}

		request.Block.ReceivedAt = msg.ReceivedAt
		request.Block.ReceivedFrom = p

		p.MarkBlock(request.Block.Hash())

		e.eventMux.Post(e2c.BlockProposal{
			Block: request.Block,
		})

		return true, nil

	} else if msg.Code == AckMsg {

		var hash common.Hash
		if err := msg.Decode(&hash); err != nil {
			return false, nil
		}

		e.eventMux.Post(e2c.Ack{
			Id:    p.String(),
			Block: hash,
		})

		return true, nil
	}
	return false, nil
}

// SetBroadcaster implements consensus.Handler.SetBroadcaster
func (e2c *E2C) SetBroadcaster(broadcaster consensus.Broadcaster) {
	e2c.broadcaster = broadcaster
}

// TODO: Figure out what this is
func (e2c *E2C) NewChainHead() error {
	return nil
}
