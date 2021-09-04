package main

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	atypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func Encode(vanity string, validators []common.Address) (string, error) {
	newVanity, err := hexutil.Decode(vanity)
	if err != nil {
		return "", err
	}

	if len(newVanity) < atypes.E2CExtraVanity {
		newVanity = append(newVanity, bytes.Repeat([]byte{0x00}, atypes.E2CExtraVanity-len(newVanity))...)
	}
	newVanity = newVanity[:atypes.E2CExtraVanity]

	ist := &atypes.E2CExtra{
		Validators: validators,
		Seal:       make([]byte, atypes.E2CExtraSeal),
	}

	payload, err := rlp.EncodeToBytes(&ist)
	if err != nil {
		return "", err
	}

	return "0x" + common.Bytes2Hex(append(newVanity, payload...)), nil
}

func main() {
	addr := []common.Address{
		common.HexToAddress("0x26519ea5fd73518efcf5ca13e6befab6836befce"),
		common.HexToAddress("0xb7c420caaccc788a5542a54c22b653c525467d8c"),
		common.HexToAddress("0x25b4c0b45f421e29a1660cfe3bb68511f7cfec26"),
		common.HexToAddress("0x1c7e5ff787dc181bf5ce7f2eb38d582e0e246350")}
	fmt.Println(Encode("0x00", addr))
	E2CDigest := common.HexToHash(crypto.Keccak256Hash([]byte("E2C practical byzantine fault tolerance")).String())
	fmt.Println(E2CDigest.String())
}
