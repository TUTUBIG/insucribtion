package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"strings"
)

const execution721PackedTx = "0xd6d09ba6a5cf3baeb3f3ee8ae7f6fef1ce6d7306089e06d1eceea412ba25d69a"
const execution721MakerFeePacked = "0x2e5e308eb742731d570b67fd02ee40c3cd1a24e2cad3ee49d293c868c9f5a054"
const execution721TakerFeePacked = "0x9a09678e0b81dc128db8b2075781e2207b644c29fc0adcd661fa552b9b418de8"

func main() {
	c, e := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/iqxe7N1WGS9PAtAjyWXalovrFIY-T114")
	if e != nil {
		panic(e)
	}
	receipt, e := c.TransactionReceipt(context.Background(), common.HexToHash(execution721PackedTx))
	if e != nil {
		panic(e)
	}

	fmt.Println(len(receipt.Logs))

	for _, l := range receipt.Logs {
		parseData(l.Topics[0].Hex(), l.Data)

	}
}

func parseData(topic string, data []byte) {
	abiData := `[ {
    "anonymous": false,
    "inputs": [
      {
        "components": [
          {
            "internalType": "address",
            "name": "trader",
            "type": "address"
          },
          {
            "internalType": "uint256",
            "name": "id",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "amount",
            "type": "uint256"
          },
          {
            "internalType": "address",
            "name": "collection",
            "type": "address"
          },
          {
            "internalType": "enum AssetType",
            "name": "assetType",
            "type": "uint8"
          }
        ],
        "indexed": false,
        "internalType": "struct Transfer",
        "name": "transfer",
        "type": "tuple"
      },
      {
        "indexed": false,
        "internalType": "bytes32",
        "name": "orderHash",
        "type": "bytes32"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "listingIndex",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "price",
        "type": "uint256"
      },
      {
        "components": [
          {
            "internalType": "address",
            "name": "recipient",
            "type": "address"
          },
          {
            "internalType": "uint16",
            "name": "rate",
            "type": "uint16"
          }
        ],
        "indexed": false,
        "internalType": "struct FeeRate",
        "name": "makerFee",
        "type": "tuple"
      },
      {
        "components": [
          {
            "components": [
              {
                "internalType": "address",
                "name": "recipient",
                "type": "address"
              },
              {
                "internalType": "uint16",
                "name": "rate",
                "type": "uint16"
              }
            ],
            "internalType": "struct FeeRate",
            "name": "protocolFee",
            "type": "tuple"
          },
          {
            "components": [
              {
                "internalType": "address",
                "name": "recipient",
                "type": "address"
              },
              {
                "internalType": "uint16",
                "name": "rate",
                "type": "uint16"
              }
            ],
            "internalType": "struct FeeRate",
            "name": "takerFee",
            "type": "tuple"
          }
        ],
        "indexed": false,
        "internalType": "struct Fees",
        "name": "fees",
        "type": "tuple"
      },
      {
        "indexed": false,
        "internalType": "enum OrderType",
        "name": "orderType",
        "type": "uint8"
      }
    ],
    "name": "Execution",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": false,
        "internalType": "bytes32",
        "name": "orderHash",
        "type": "bytes32"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "tokenIdListingIndexTrader",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "collectionPriceSide",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "makerFeeRecipientRate",
        "type": "uint256"
      }
    ],
    "name": "Execution721MakerFeePacked",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": false,
        "internalType": "bytes32",
        "name": "orderHash",
        "type": "bytes32"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "tokenIdListingIndexTrader",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "collectionPriceSide",
        "type": "uint256"
      }
    ],
    "name": "Execution721Packed",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": false,
        "internalType": "bytes32",
        "name": "orderHash",
        "type": "bytes32"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "tokenIdListingIndexTrader",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "collectionPriceSide",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "takerFeeRecipientRate",
        "type": "uint256"
      }
    ],
    "name": "Execution721TakerFeePacked",
    "type": "event"
  }]`
	parsedAbi, err := abi.JSON(strings.NewReader(abiData))
	if err != nil {
		panic(err)
	}

	type Transfer struct {
		Trader     common.Address
		Id         *big.Int
		Amount     *big.Int
		Collection common.Address
		AssetType  uint8 // 假设 AssetType 是 uint8 类型
	}

	type FeeRate struct {
		Recipient common.Address
		Rate      uint16
	}

	type Fees struct {
		ProtocolFee FeeRate
		TakerFee    FeeRate
	}

	type Execution struct {
		TransferData Transfer
		OrderHash    [32]byte
		ListingIndex *big.Int
		Price        *big.Int
		MakerFee     FeeRate
		FeesData     Fees
		OrderType    uint8
	}

	type Execution721Packed struct {
		OrderHash                 [32]byte
		TokenIdListingIndexTrader *big.Int
		CollectionPriceSide       *big.Int
	}

	type Execution721TakerFeePacked struct {
		OrderHash                 [32]byte
		TokenIdListingIndexTrader *big.Int
		CollectionPriceSide       *big.Int
		TakerFeeRecipientRate     *big.Int
	}

	type Execution721MakerFeePacked struct {
		OrderHash                 [32]byte
		TokenIdListingIndexTrader *big.Int
		CollectionPriceSide       *big.Int
		MakerFeeRecipientRate     *big.Int
	}

	switch topic {
	case "":
		event := new(Execution)
		if err = parsedAbi.UnpackIntoInterface(event, "Execution", data); err != nil {
			return
		}
		fmt.Println(event)
	case "0x1d5e12b51dee5e4d34434576c3fb99714a85f57b0fd546ada4b0bddd736d12b2":
		event := new(Execution721Packed)
		if err = parsedAbi.UnpackIntoInterface(event, "Execution721Packed", data); err != nil {
			return
		}
		fmt.Println(event)
		blurParsePacked(event.CollectionPriceSide)
	case "0x0fcf17fac114131b10f37b183c6a60f905911e52802caeeb3e6ea210398b81ab":
		event := new(Execution721TakerFeePacked)
		if err = parsedAbi.UnpackIntoInterface(event, "Execution721TakerFeePacked", data); err != nil {
			return
		}
		fmt.Println(event)
		blurParsePacked(event.CollectionPriceSide)
	case "0x7dc5c0699ac8dd5250cbe368a2fc3b4a2daadb120ad07f6cccea29f83482686e":
		event := new(Execution721MakerFeePacked)
		if err = parsedAbi.UnpackIntoInterface(event, "Execution721MakerFeePacked", data); err != nil {
			return
		}
		fmt.Println(event)
		blurParsePacked(event.CollectionPriceSide)
	}

}

func blurParsePacked(collectionPriceSide *big.Int) {
	fmt.Println(collectionPriceSide.String())
	mask88 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 88), big.NewInt(1))

	p := new(big.Int).Rsh(collectionPriceSide, 160)
	p.And(p, mask88)

	fmt.Println("price", p.String())
}
