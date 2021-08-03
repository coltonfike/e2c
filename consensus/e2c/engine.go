// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package e2c

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Backend provides application specific functions for E2C core
type Backend interface {
	// Address returns the owner's address
	Address() common.Address

	// Returns the current Leader
	Leader() common.Address

	// EventMux returns the event mux in backend
	EventMux() *event.TypeMux

	// Broadcast sends a message to all peers
	Broadcast(payload []byte) error

	// Sends a new block to all peers
	SendNewBlock(*types.Block) error

	// Relays a block header to all peers
	RelayBlock(common.Hash) error

	// Sends blame message to all peers
	SendBlame() error
	SendVote(*types.Block, common.Address) error
	SendBlameCertificate(BlameCertificate) error

	// Requests a block from peers
	RequestBlock(common.Hash, common.Address) error

	// Responds to a request for a block
	RespondToRequest(*types.Block, common.Address) error

	// The block will be put into blockchain.
	Commit(*types.Block)

	// Verify verifies the block is valid
	Verify(*types.Block) error

	GetBlockFromChain(common.Hash) (*types.Block, error)

	// Sign signs input data with the backend's private key
	Sign([]byte) ([]byte, error)

	ChangeView()
}

type Engine interface {
	Start(*types.Block) error
	Stop() error
	GetQueuedBlock(common.Hash) (*types.Header, error)
	Lock() *types.Block
}
