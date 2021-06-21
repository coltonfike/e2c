package e2c

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

type Engine interface {
	EventMux() *event.TypeMux
	Verify(*types.Header) error
	Commit(*types.Block) error
	BroadcastBlock(*types.Block)
	BroadcastBlame()
	RequestBlock(*common.Hash)
}
