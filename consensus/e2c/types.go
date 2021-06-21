package e2c

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type BlockProposal struct {
	Id    string
	Block *types.Block
}

type Ack struct {
	Id    string
	Block common.Hash
}
