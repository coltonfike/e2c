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
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/p2p"
)

// HandleMsg implements consensus.Handler.HandleMsg
func (e2c *E2C) HandleMsg(addr common.Address, msg p2p.Msg) (bool, error) {
	// verify message by just copying the stuff from handler
	return false, nil
}

// SetBroadcaster implements consensus.Handler.SetBroadcaster
func (e2c *E2C) SetBroadcaster(broadcaster consensus.Broadcaster) {
	e2c.broadcaster = broadcaster
}

func (e2c *E2C) NewChainHead() error {
	return nil
}

// TODO: Commit all known ancestors of the block
func (e2c *E2C) HandleNewBlock(block *types.Block, id string, mark func(common.Hash), hash common.Hash) (bool, error) {

	fmt.Println("Block " + block.Number().String() + " Received! Broadcasting to peers...")
	e2c.broadcaster.BroadcastBlock(block, false)
	timer := time.NewTimer(1 * time.Second)

	<-timer.C
	fmt.Println("Committing Block " + block.Number().String() + "\n")
	mark(hash)
	e2c.broadcaster.Enqueue(id, block)

	return false, nil
}
