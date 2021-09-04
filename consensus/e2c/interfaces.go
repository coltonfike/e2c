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

	// Return the set of validators
	Validators() Validators

	// Returns F for the valset
	F() uint64

	// Returns the state the system is, Steady-State, View Change, etc
	Status() uint32

	// Changes the status the node is in
	SetStatus(uint32)

	// Returns the view
	View() uint64

	// EventMux returns the event mux in backend
	EventMux() *event.TypeMux

	// Broadcast sends a message to all peers
	Broadcast([]byte) error

	// Send sends a message to a single peer
	Send([]byte, common.Address) error

	// commit adds the block to the chain
	Commit(*types.Block)

	// Verify verifies the block is valid
	Verify(*types.Block) error

	// This is used by core to access a block from the chain
	GetBlockFromChain(common.Hash) (*types.Block, error)

	// Sign signs input data with the backend's private key
	Sign([]byte) ([]byte, error)

	// Triggers a view change
	ChangeView()
}

type Engine interface {
	// Starts the engine by initializing variables
	Start(*types.Block) error

	// Stops the engine by clearing memory/writing to caches
	Stop() error

	// Returns blocks that are currently in the queue
	GetQueuedBlock(common.Hash) (*types.Header, error)
	Propose(*types.Block) error
}
