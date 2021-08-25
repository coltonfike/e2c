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
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// Message codes
const (
	NewBlockMsg uint64 = iota
	BlameMsg
	ValidateMsg
	BlameCertificateMsg
	BlockCertificateMsg
	FirstProposalMsg
	SecondProposalMsg
	VoteMsg
	RequestBlockMsg
	RespondMsg
)

type Message struct {
	Code      uint64
	Msg       []byte
	View      uint64
	Address   common.Address
	Signature []byte
}

// ==============================================
//
// define the functions that needs to be provided for rlp Encoder/Decoder.

// EncodeRLP serializes m into the Ethereum RLP format.
func (m *Message) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{m.Code, m.Msg, m.View, m.Address, m.Signature})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (m *Message) DecodeRLP(s *rlp.Stream) error {
	var msg struct {
		Code      uint64
		Msg       []byte
		View      uint64
		Address   common.Address
		Signature []byte
	}

	if err := s.Decode(&msg); err != nil {
		return err
	}
	m.Code, m.Msg, m.View, m.Address, m.Signature = msg.Code, msg.Msg, msg.View, msg.Address, msg.Signature
	return nil
}

// ==============================================
//
// define the functions that needs to be provided for core.

func (m *Message) FromPayload(b []byte, validateFn func([]byte, []byte) (common.Address, error)) error {
	// Decode Message
	err := rlp.DecodeBytes(b, &m)
	if err != nil {
		return err
	}

	return m.VerifySig(validateFn)
}

func (m *Message) VerifySig(validateFn func([]byte, []byte) (common.Address, error)) (err error) {
	// Validate Message (on a Message without Signature)
	var payload []byte
	payload, err = m.PayloadNoSig()
	if err != nil {
		return err
	}

	// ensure message came from validators
	addr, err := validateFn(payload, m.Signature)
	if err != nil {
		return err
	}
	if !bytes.Equal(addr.Bytes(), m.Address.Bytes()) {
		return errors.New("invalid signer")
		//return errInvalidSigner @todo add this error
	}
	return nil
}

// Sign the message
func (m *Message) Sign(sign func([]byte) ([]byte, error)) error {
	data, err := m.PayloadNoSig()
	if err != nil {
		return err
	}
	m.Signature, err = sign(data)
	if err != nil {
		return err
	}
	return nil
}

// Converts the message into []byte
func (m *Message) Payload() ([]byte, error) {
	return rlp.EncodeToBytes(m)
}

// returns the []byte of message with an empty sig field for signing
func (m *Message) PayloadNoSig() ([]byte, error) {
	return rlp.EncodeToBytes(&Message{
		Code:      m.Code,
		Msg:       m.Msg,
		View:      m.View,
		Address:   m.Address, // @todo when changing to more memory efficient signing, make this empty?
		Signature: []byte{},
	})
}

// retrieve payload with a Signature included
func (m *Message) PayloadWithSig(sign func([]byte) ([]byte, error)) ([]byte, error) {
	if err := m.Sign(sign); err != nil {
		return nil, err
	}
	return m.Payload()
}

// Decodes the message.Msg field into val
func (m *Message) Decode(val interface{}) error {
	return rlp.DecodeBytes(m.Msg, val)
}

func (m *Message) String() string {
	return fmt.Sprintf("{Code: %v, Address: %v}", m.Code, m.Address.String())
}

// Encodes val into a []byte for the message.Msg field. Note that val must be able to be RLP encoded
func Encode(val interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(val)
}

// ensures the message is from correct view
func (c *core) verifyMsg(msg *Message) error {
	if msg.View != c.backend.View() && !(msg.Code == RequestBlockMsg || msg.Code == RespondMsg) {
		return errors.New("msg from different view")
	}
	return nil
}
