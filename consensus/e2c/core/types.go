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

package core

import (
	"io"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// All of these are structs for sending messages with more data than one field
// Each of them includes method for RLP encoding/decoding

type EquivBlame struct {
	Blame *Message
	B1    *types.Block
	B2    *types.Block
}

func (b *EquivBlame) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.Blame, b.B1, b.B2})
}

func (b *EquivBlame) DecodeRLP(s *rlp.Stream) error {
	var blame struct {
		Blame *Message
		B1    *types.Block
		B2    *types.Block
	}

	if err := s.Decode(&blame); err != nil {
		return err
	}
	b.Blame, b.B1, b.B2 = blame.Blame, blame.B1, blame.B2
	return nil
}

type Vote struct {
	Blocks []*Message
}

func (v *Vote) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{v.Blocks})
}

func (v *Vote) DecodeRLP(s *rlp.Stream) error {
	var vote struct {
		Blocks []*Message
	}

	if err := s.Decode(&vote); err != nil {
		return err
	}
	v.Blocks = vote.Blocks
	return nil
}

type BlockCertificate struct {
	Block *types.Block
	Votes [][]byte
}

func (bc *BlockCertificate) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{bc.Block, bc.Votes})
}

func (bc *BlockCertificate) DecodeRLP(s *rlp.Stream) error {
	var cert struct {
		Block *types.Block
		Votes [][]byte
	}

	if err := s.Decode(&cert); err != nil {
		return err
	}
	bc.Block, bc.Votes = cert.Block, cert.Votes
	return nil
}

type FirstProposal struct {
	Cert  *BlockCertificate
	Block *types.Block
}

func (b *FirstProposal) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.Cert, b.Block})
}

func (b *FirstProposal) DecodeRLP(s *rlp.Stream) error {
	var cert struct {
		Cert  *BlockCertificate
		Block *types.Block
	}

	if err := s.Decode(&cert); err != nil {
		return err
	}
	b.Cert, b.Block = cert.Cert, cert.Block
	return nil
}

type SecondProposal struct {
	Validates [][]byte
	Block     *types.Block
}

func (b *SecondProposal) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.Validates, b.Block})
}

func (b *SecondProposal) DecodeRLP(s *rlp.Stream) error {
	var cert struct {
		Validates [][]byte
		Block     *types.Block
	}

	if err := s.Decode(&cert); err != nil {
		return err
	}
	b.Validates, b.Block = cert.Validates, cert.Block
	return nil
}
