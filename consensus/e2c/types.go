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
	"io"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type NewBlockEvent struct {
	Block *types.Block
}

type RelayBlockEvent struct {
	Hash    common.Hash
	Address common.Address
}

type BlameEvent struct {
	Time    time.Time
	Address common.Address
}
type RequestBlockEvent struct {
	Hash    common.Hash
	Address common.Address
}

type RespondToRequestEvent struct {
	Block *types.Block
}

type Vote struct {
	Block   *types.Block
	Address common.Address
}

type BlameCertificateEvent struct {
	Lock      *types.Block
	Committed *types.Block
	Address   common.Address
}

type BlameCertificate struct {
	Lock      *types.Block
	Committed *types.Block
}

func (bc *BlameCertificate) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{bc.Lock, bc.Committed})
}

func (bc *BlameCertificate) DecodeRLP(s *rlp.Stream) error {
	var cert struct {
		Lock      *types.Block
		Committed *types.Block
	}

	if err := s.Decode(&cert); err != nil {
		return err
	}
	bc.Lock, bc.Committed = cert.Lock, cert.Committed
	return nil
}
