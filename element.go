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

const tx = "0x3eefdb377fdad70618c2e1673f3eb72b1e3ae3de67f25b1a9c5ce17235484979"

func main() {
	c, e := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/iqxe7N1WGS9PAtAjyWXalovrFIY-T114")
	if e != nil {
		panic(e)
	}
	receipt, e := c.TransactionReceipt(context.Background(), common.HexToHash(tx))
	if e != nil {
		panic(e)
	}

	fmt.Println(len(receipt.Logs))

	for _, l := range receipt.Logs {
		parseData(l.Topics[0].Hex(), l.Data)

	}
}

func parseData(topic string, data []byte) {
	abiData := `[
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
        "internalType": "address",
        "name": "maker",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "taker",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "nonce",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "erc20Token",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "erc20TokenAmount",
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
            "internalType": "uint256",
            "name": "amount",
            "type": "uint256"
          }
        ],
        "indexed": false,
        "internalType": "struct FeeItem[]",
        "name": "fees",
        "type": "tuple[]"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "erc721Token",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "erc721TokenId",
        "type": "uint256"
      }
    ],
    "name": "ERC721SellOrderFilled",
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
        "internalType": "address",
        "name": "maker",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "taker",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "nonce",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "erc20Token",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "erc20TokenAmount",
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
            "internalType": "uint256",
            "name": "amount",
            "type": "uint256"
          }
        ],
        "indexed": false,
        "internalType": "struct FeeItem[]",
        "name": "fees",
        "type": "tuple[]"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "erc721Token",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "erc721TokenId",
        "type": "uint256"
      }
    ],
    "name": "ERC721BuyOrderFilled",
    "type": "event"
  }
]`
	parsedAbi, err := abi.JSON(strings.NewReader(abiData))
	if err != nil {
		panic(err)
	}

	type FeeItem struct {
		Recipient common.Address
		Amount    *big.Int
	}

	type ERC721OrderFilledEvent struct {
		OrderHash        [32]byte
		Maker            common.Address
		Taker            common.Address
		Nonce            *big.Int
		Erc20Token       common.Address
		Erc20TokenAmount *big.Int
		Fees             []FeeItem
		Erc721Token      common.Address
		Erc721TokenId    *big.Int
	}

	event := new(ERC721OrderFilledEvent)
	switch topic {
	case "0x9c248aa1a265aa616f707b979d57f4529bb63a4fc34dc7fc61fdddc18410f74e":
		err = parsedAbi.UnpackIntoInterface(event, "ERC721SellOrderFilled", data)
		if err != nil {
			panic(err)
		}
	case "":
		err = parsedAbi.UnpackIntoInterface(event, "ERC721BuyOrderFilled", data)
		if err != nil {
			panic(err)
		}
	default:
		return
	}

	fmt.Println("price", event.Erc20TokenAmount.String())
	fmt.Println("fee", event.Fees[0].Amount)
}
