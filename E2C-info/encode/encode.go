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
		common.HexToAddress("0396ea1512b97c9f7e90f641a46f48967db064ba"),
		common.HexToAddress("17145655faf1fcbbd0473502c504f32ca9ebe144"),
		common.HexToAddress("1a80a2887c640c886606e34d9cfc48637a5b4ceb"),
		common.HexToAddress("200a7b1d5f2de512c05b0c46eb50e4d2f2922ada"),
		common.HexToAddress("2b1b9762f468edc9c296cc5b0023d2059d1355f4"),
		common.HexToAddress("302fd725344392baa47a1314b7991c090f51b54d"),
		common.HexToAddress("3899eeba191fd1fff68812b04273e67c91691133"),
		common.HexToAddress("43708b153d51f101858aff92118bc74a1742c9a9"),
		common.HexToAddress("499ff308159bc686af7db92990b1f22b8dc94bcf"),
		common.HexToAddress("49f00383b9ee4096223d0af74044510d27d66faf"),
		common.HexToAddress("4b0999ad9abb26399c75e279f5c4d0273ad3864e"),
		common.HexToAddress("4d083558d07c534d3485d5617d54d3f63dd2a851"),
		common.HexToAddress("5544a7093e47d31a585ce5f54b45658e021a6b0b"),
		common.HexToAddress("5c5f8f90aa573f39dac11f6a5028619cdb926a5d"),
		common.HexToAddress("70b7b4db2474330c181223e744be8d9d410bfe6f"),
		common.HexToAddress("82fe2501a7fe7ee3b0e4c28f4651be7082056292"),
		common.HexToAddress("8aa6b5a6a1653245f5b17956e7748614cf1565ff"),
		common.HexToAddress("8cfd2effa72b8765e8b0d26e4f1221922acfb0e7"),
		common.HexToAddress("971d22a541cf6bee1c8feb33100b29f8a87a56d0"),
		common.HexToAddress("aab34d5eecb1d496cdcc4a3338785da8f847c78c"),
		common.HexToAddress("b25930d7d3d0878ccde35eebfe06401d8f464705"),
		common.HexToAddress("b2d2f006ff3acf88223db534f6f9ac224473125e"),
		common.HexToAddress("ccfafd3d812924773f0bb81a29669dde3d6d9cad"),
		common.HexToAddress("cef71810d7178c423af65a12606cc0b39e2bcc8d"),
		common.HexToAddress("d2d9b44f1287754ea85279f4cb1814d2f29f32b5"),
		common.HexToAddress("d38f4448a8e9276315a7c0bd562a4f6b0ee44dab"),
		common.HexToAddress("e8563f0f94cc4b1c9f8001a83a3aec1fda9499d2"),
		common.HexToAddress("eca7f9c787dc31c40236cbbc2ab773b3d20a0679"),
		common.HexToAddress("f1cee6ceab1bde33e93a1ba14b9bf0f6fb49f414"),
		common.HexToAddress("f2ed520ee0db385913556cbb0a0fbe29e6d132dc"),
		common.HexToAddress("f56eb64aa7c10cf173bdd50760d97acae3fdb556"),
		common.HexToAddress("f86121bba81358c9445cba9d5b9b0936b6270539")}

	fmt.Println(Encode("0x00", addr))
	E2CDigest := common.HexToHash(crypto.Keccak256Hash([]byte("E2C practical byzantine fault tolerance")).String())
	fmt.Println(E2CDigest.String())
}
