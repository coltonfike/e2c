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

package backend

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/p2p"
	lru "github.com/hashicorp/golang-lru"
)

const (
	e2cMsg            = 0x11
	NewBlockMsg       = 0x07
	NewBlockHashesMsg = 0x01
)

var (
	// errDecodeFailed is returned when decode message fails
	errDecodeFailed = errors.New("fail to decode e2c message")
)

// Protocol implements consensus.Engine.Protocol
func (b *backend) Protocol() consensus.Protocol {
	return consensus.IstanbulProtocol
}

func (b *backend) decode(msg p2p.Msg) ([]byte, common.Hash, error) {
	var data []byte
	if err := msg.Decode(&data); err != nil {
		return nil, common.Hash{}, errDecodeFailed
	}

	return data, e2c.RLPHash(data), nil
}

// HandleMsg implements consensus.Handler.HandleMsg
func (b *backend) HandleMsg(addr common.Address, msg p2p.Msg) (bool, error) {
	b.coreMu.Lock()
	defer b.coreMu.Unlock()
	if msg.Code == e2cMsg && b.coreStarted {

		data, hash, err := b.decode(msg)
		if err != nil {
			return true, errDecodeFailed
		}
		// Mark peer's message
		ms, ok := b.recentMessages.Get(addr)
		var m *lru.ARCCache
		if ok {
			m, _ = ms.(*lru.ARCCache)
		} else {
			m, _ = lru.NewARC(inmemoryMessages)
			b.recentMessages.Add(addr, m)
		}
		m.Add(hash, true)

		// Mark self known message
		if _, ok := b.knownMessages.Get(hash); ok {
			return true, nil
		}
		b.knownMessages.Add(hash, true)

		msg := new(message)
		if err := msg.FromPayload(data); err != nil {
			return true, err
		}

		switch msg.Code {
		case newBlockMsgCode:
			// If this message isn't from the leader, then drop the peer
			if msg.Address != b.leader {
				return true, errUnauthorized
			}

			var block *types.Block
			if err := msg.Decode(&block); err != nil {
				return true, err
			}
			b.eventMux.Post(e2c.NewBlockEvent{Block: block})

		case relayMsgCode:

			var hash common.Hash
			if err := msg.Decode(&hash); err != nil {
				return true, err
			}
			b.eventMux.Post(e2c.RelayBlockEvent{Hash: hash, Address: msg.Address})

		case blameMsgCode:

			var t time.Time
			if err := msg.Decode(&t); err != nil {
				return true, err
			}
			b.eventMux.Post(e2c.BlameEvent{Time: t, Address: msg.Address})

		case requestBlockMsgCode:

			var request common.Hash
			if err := msg.Decode(&request); err != nil {
				return true, err
			}
			b.eventMux.Post(e2c.RequestBlockEvent{Hash: request, Address: msg.Address})

		case respondToRequestMsgCode:

			var block *types.Block
			if err := msg.Decode(&block); err != nil {
				return true, err
			}
			b.eventMux.Post(e2c.RespondToRequestEvent{Block: block})
		}

		return true, nil
	}
	//@todo, both of these lower ones need to be adjusted so that we only reject ones that we handle
	// We commit our own blocks, and thus, don't want this to run on the protocol manager
	if msg.Code == NewBlockMsg {
		// ignore if we aren't a client node
		if b.coreStarted {
			return true, nil
		}
		// @todo wait for so many acks before committing
		var request struct {
			Block *types.Block
			TD    *big.Int
		}
		if err := msg.Decode(&request); err != nil {
			fmt.Println(err)
			return true, err
		}

		if n, ok := b.clientBlocks[request.Block.Hash()]; ok {
			b.clientBlocks[request.Block.Hash()] = n + 1
		} else {
			b.clientBlocks[request.Block.Hash()] = 1
		}

		fmt.Println("Block received. Total acks for block", request.Block.Number().String(), ":", b.clientBlocks[request.Block.Hash()])
		if b.clientBlocks[request.Block.Hash()] >= b.config.F {
			b.Commit(request.Block)

			time.AfterFunc(3*b.config.Delta*time.Millisecond, func() {
				delete(b.clientBlocks, request.Block.Hash())
			})
			fmt.Println("Client committed block", request.Block.Number().String())
		}
		return true, nil
	}
	// Likewise, we reject block header announcements sent by fetcher
	if msg.Code == NewBlockHashesMsg {
		if b.coreStarted {
			return true, nil
		}
		// @todo wait for so many acks before committing
		return true, nil
	}
	return false, nil
}

// SetBroadcaster implements consensus.Handler.SetBroadcaster
func (b *backend) SetBroadcaster(broadcaster consensus.Broadcaster) {
	b.broadcaster = broadcaster
}

// NewChainHead implements consensus.Handler.SetBroadcaster
// It's useless in E2C
func (b *backend) NewChainHead() error {
	return nil
}
