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
	"bytes"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	newBlockMsgCode uint64 = iota
	relayMsgCode
	blameMsgCode
	validateMsgCode
	blameCertCode
	blockCertMsgCode
	newBlockOneMsgCode
	finalBlockMsgCode
	voteMsgCode
	requestBlockMsgCode
	respondToRequestMsgCode
)

type message struct {
	Code      uint64
	Msg       []byte
	Address   common.Address
	Signature []byte
}

// ==============================================
//
// define the functions that needs to be provided for rlp Encoder/Decoder.

// EncodeRLP serializes m into the Ethereum RLP format.
func (m *message) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{m.Code, m.Msg, m.Address, m.Signature})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (m *message) DecodeRLP(s *rlp.Stream) error {
	var msg struct {
		Code      uint64
		Msg       []byte
		Address   common.Address
		Signature []byte
	}

	if err := s.Decode(&msg); err != nil {
		fmt.Println(err)
		return err
	}
	m.Code, m.Msg, m.Address, m.Signature = msg.Code, msg.Msg, msg.Address, msg.Signature
	return nil
}

// ==============================================
//
// define the functions that needs to be provided for core.

func (m *message) FromPayload(b []byte) error {
	// Decode Message
	err := rlp.DecodeBytes(b, &m)
	if err != nil {
		return err
	}

	// we don't sign relay messages, so no need to validate
	if m.Code == relayMsgCode || m.Code == requestBlockMsgCode || m.Code == respondToRequestMsgCode {
		return nil
	}
	// Validate Message (on a Message without Signature)
	var payload []byte
	payload, err = m.PayloadNoSig()
	if err != nil {
		return err
	}

	addr, err := e2c.GetSignatureAddress(payload, m.Signature)
	if err != nil {
		return err
	}
	if !bytes.Equal(addr.Bytes(), m.Address.Bytes()) {
		return err
	}
	return nil
}

func (m *message) Payload() ([]byte, error) {
	return rlp.EncodeToBytes(m)
}

func (m *message) PayloadNoSig() ([]byte, error) {
	return rlp.EncodeToBytes(&message{
		Code:      m.Code,
		Msg:       m.Msg,
		Address:   m.Address,
		Signature: []byte{},
	})
}

func (m *message) PayloadWithSig(sign func([]byte) ([]byte, error)) ([]byte, error) {
	data, err := m.PayloadNoSig()
	if err != nil {
		return nil, err
	}
	m.Signature, err = sign(data)
	if err != nil {
		return nil, err
	}
	return m.Payload()
}

func (m *message) Decode(val interface{}) error {
	return rlp.DecodeBytes(m.Msg, val)
}

func (m *message) String() string {
	return fmt.Sprintf("{Code: %v, Address: %v}", m.Code, m.Address.String())
}

func Encode(val interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(val)
}
