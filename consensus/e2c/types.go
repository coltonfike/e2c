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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type NewBlockEvent struct {
	Block *types.Block
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (e *NewBlockEvent) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{e.Block})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (e *NewBlockEvent) DecodeRLP(s *rlp.Stream) error {
	var block struct {
		Block *types.Block
	}

	if err := s.Decode(&block); err != nil {
		return err
	}
	e.Block = block.Block
	return nil
}

type RelayBlockEvent struct {
	Header *types.Header
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (e *RelayBlockEvent) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{e.Header})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (e *RelayBlockEvent) DecodeRLP(s *rlp.Stream) error {
	var header struct {
		Header *types.Header
	}

	if err := s.Decode(&header); err != nil {
		return err
	}
	e.Header = header.Header
	return nil
}

type BlameEvent struct {
	Address common.Address
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (e *BlameEvent) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{e.Address})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (e *BlameEvent) DecodeRLP(s *rlp.Stream) error {
	var addr struct {
		Address common.Address
	}

	if err := s.Decode(&addr); err != nil {
		return err
	}
	e.Address = addr.Address
	return nil
}
