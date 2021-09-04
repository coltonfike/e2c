package backend

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
)

// this is called by eth.Handler. We count how many acks we have received and return true when the block should be committed
func (b *backend) ClientVerify(block *types.Block, addr common.Address, chain consensus.ChainHeaderReader) bool {
	b.clientMu.Lock()
	defer b.clientMu.Unlock()

	if b.Validators() == nil {
		header := chain.CurrentHeader()
		e2cExtra, err := types.ExtractE2CExtra(header)
		if err != nil {
			return false
		}
		b.validators = e2cExtra.Validators
	}

	if i, _ := b.Validators().GetByAddress(addr); i == -1 {
		return false
	}

	if n, ok := b.clientBlocks[block.Hash()]; ok {
		b.clientBlocks[block.Hash()] = n + 1
	} else {
		b.clientBlocks[block.Hash()] = 1
	}

	b.logger.Info("[E2C] Block acknowledgement received", "number", block.Number(), "hash", block.Hash(), "total acks", b.clientBlocks[block.Hash()])
	if b.clientBlocks[block.Hash()] == b.F()+1 {
		// Delete the parent block. If we delete the one we just committed
		// then we will probably see it again since we commit at F+1 acks
		// This means it doesn't actually get delete and we will run into
		// Memory errors. So instead we delete the parent since we shouldn't
		// See that block anymore. We could add a timeout function to delete
		// after a set interval
		delete(b.clientBlocks, block.ParentHash())
		b.logger.Info("[E2C] Client committed block", "number", block.Number(), "hash", block.Hash())
		return true
	}

	return false
}
