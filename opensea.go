package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// OfferItem 结构体
type OfferItem struct {
	ItemType             uint8
	Token                common.Address
	IdentifierOrCriteria *big.Int
	Amount               *big.Int
}

// ConsiderationItem 结构体
type ConsiderationItem struct {
	ItemType             uint8
	Token                common.Address
	IdentifierOrCriteria *big.Int
	Amount               *big.Int
	Recipient            common.Address
}

type OrderFulfilledEvent struct {
	OrderHash     [32]byte
	Offerer       common.Address
	Zone          common.Address
	Recipient     common.Address
	Offer         []OfferItem
	Consideration []ConsiderationItem
}

func main() {

	c, e := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/iqxe7N1WGS9PAtAjyWXalovrFIY-T114")
	if e != nil {
		panic(e)
	}
	receipt, e := c.TransactionReceipt(context.Background(), common.HexToHash("0x465f34dcee32751d2a34253efccab8486b127b208e7f288ff1321d5487ce9e12"))
	if e != nil {
		panic(e)
	}

	fmt.Println(len(receipt.Logs))

	for _, l := range receipt.Logs {
		if l.Topics[0].Hex() == "0x9d9af8e38d66c62e2c12f0225249fd9d721c54b83f48d9352c97c6cacdcb6f31" {
			parseData(l.Data)
		}
	}

}

func parseData(data []byte) {
	// ABI 字符串
	abiData := `[{
    "anonymous": false,
    "inputs": [
      {
        "indexed": false,
        "internalType": "bytes32",
        "name": "orderHash",
        "type": "bytes32"
      },
      {
        "indexed": true,
        "internalType": "address",
        "name": "offerer",
        "type": "address"
      },
      {
        "indexed": true,
        "internalType": "address",
        "name": "zone",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "recipient",
        "type": "address"
      },
      {
        "components": [
          {
            "internalType": "enum ItemType",
            "name": "itemType",
            "type": "uint8"
          },
          {
            "internalType": "address",
            "name": "token",
            "type": "address"
          },
          {
            "internalType": "uint256",
            "name": "identifier",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "amount",
            "type": "uint256"
          }
        ],
        "indexed": false,
        "internalType": "struct SpentItem[]",
        "name": "offer",
        "type": "tuple[]"
      },
      {
        "components": [
          {
            "internalType": "enum ItemType",
            "name": "itemType",
            "type": "uint8"
          },
          {
            "internalType": "address",
            "name": "token",
            "type": "address"
          },
          {
            "internalType": "uint256",
            "name": "identifier",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "amount",
            "type": "uint256"
          },
          {
            "internalType": "address payable",
            "name": "recipient",
            "type": "address"
          }
        ],
        "indexed": false,
        "internalType": "struct ReceivedItem[]",
        "name": "consideration",
        "type": "tuple[]"
      }
    ],
    "name": "OrderFulfilled",
    "type": "event"
  }]` // ABI 字符串，您需要从合约的 ABI 中获取
	parsedAbi, err := abi.JSON(strings.NewReader(abiData))
	if err != nil {
		panic(err)
	}

	// 解析日志
	event := new(OrderFulfilledEvent)
	err = parsedAbi.UnpackIntoInterface(event, "OrderFulfilled", data)
	if err != nil {
		panic(err)
	}

	fmt.Printf("OrderFulfilled Event: %s\n", hex.EncodeToString(event.OrderHash[:]))
	for i := range event.Offer {
		fmt.Println("offer", event.Offer[i].IdentifierOrCriteria, event.Offer[i].Amount, event.Offer[i].Token)
	}

	for i := range event.Consideration {
		fmt.Println("consider", event.Consideration[i].IdentifierOrCriteria, event.Consideration[i].Amount, event.Consideration[i].Token)
	}
}
