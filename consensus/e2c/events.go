package e2c

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type BlockProposal struct {
	id    string
	block *types.Block
}

type Ack struct {
	id    string
	block common.Hash
}
