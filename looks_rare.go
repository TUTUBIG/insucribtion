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

const bidTx = "0x9a09678e0b81dc128db8b2075781e2207b644c29fc0adcd661fa552b9b418de8"
const askTx = "0x9254662c1e4554dd1a6c868f68f3753f4439416380a118b09198d715025405a7"

func main() {
	c, e := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/iqxe7N1WGS9PAtAjyWXalovrFIY-T114")
	if e != nil {
		panic(e)
	}
	receipt, e := c.TransactionReceipt(context.Background(), common.HexToHash(bidTx))
	if e != nil {
		panic(e)
	}

	fmt.Println(len(receipt.Logs))

	for _, l := range receipt.Logs {
		parseData(l.Topics[0].Hex(), l.Data)

	}
}

func parseData(topic string, data []byte) {
	abiData := `[{
    "anonymous": false,
    "inputs": [
      {
        "components": [
          {
            "internalType": "bytes32",
            "name": "orderHash",
            "type": "bytes32"
          },
          {
            "internalType": "uint256",
            "name": "orderNonce",
            "type": "uint256"
          },
          {
            "internalType": "bool",
            "name": "isNonceInvalidated",
            "type": "bool"
          }
        ],
        "indexed": false,
        "internalType": "struct ILooksRareProtocol.NonceInvalidationParameters",
        "name": "nonceInvalidationParameters",
        "type": "tuple"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "bidUser",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "bidRecipient",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "strategyId",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "currency",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "collection",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256[]",
        "name": "itemIds",
        "type": "uint256[]"
      },
      {
        "indexed": false,
        "internalType": "uint256[]",
        "name": "amounts",
        "type": "uint256[]"
      },
      {
        "indexed": false,
        "internalType": "address[2]",
        "name": "feeRecipients",
        "type": "address[2]"
      },
      {
        "indexed": false,
        "internalType": "uint256[3]",
        "name": "feeAmounts",
        "type": "uint256[3]"
      }
    ],
    "name": "TakerBid",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "components": [
          {
            "internalType": "bytes32",
            "name": "orderHash",
            "type": "bytes32"
          },
          {
            "internalType": "uint256",
            "name": "orderNonce",
            "type": "uint256"
          },
          {
            "internalType": "bool",
            "name": "isNonceInvalidated",
            "type": "bool"
          }
        ],
        "indexed": false,
        "internalType": "struct ILooksRareProtocol.NonceInvalidationParameters",
        "name": "nonceInvalidationParameters",
        "type": "tuple"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "askUser",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "bidUser",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "strategyId",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "currency",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "address",
        "name": "collection",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256[]",
        "name": "itemIds",
        "type": "uint256[]"
      },
      {
        "indexed": false,
        "internalType": "uint256[]",
        "name": "amounts",
        "type": "uint256[]"
      },
      {
        "indexed": false,
        "internalType": "address[2]",
        "name": "feeRecipients",
        "type": "address[2]"
      },
      {
        "indexed": false,
        "internalType": "uint256[3]",
        "name": "feeAmounts",
        "type": "uint256[3]"
      }
    ],
    "name": "TakerAsk",
    "type": "event"
  }
]`
	parsedAbi, err := abi.JSON(strings.NewReader(abiData))
	if err != nil {
		panic(err)
	}

	type NonceInvalidationParameters struct {
		OrderHash          [32]byte
		OrderNonce         *big.Int
		IsNonceInvalidated bool
	}

	type TakerBidEvent struct {
		NonceInvalidationParameters NonceInvalidationParameters
		BidUser                     common.Address
		BidRecipient                common.Address
		StrategyId                  *big.Int
		Currency                    common.Address
		Collection                  common.Address
		ItemIds                     []*big.Int
		Amounts                     []*big.Int
		FeeRecipients               [2]common.Address
		FeeAmounts                  [3]*big.Int
	}
	type TakerAskEvent struct {
		NonceInvalidationParameters NonceInvalidationParameters
		AskUser                     common.Address
		BidUser                     common.Address
		StrategyId                  *big.Int
		Currency                    common.Address
		Collection                  common.Address
		ItemIds                     []*big.Int
		Amounts                     []*big.Int
		FeeRecipients               [2]common.Address
		FeeAmounts                  [3]*big.Int
	}
	switch topic {
	case "0x3ee3de4684413690dee6fff1a0a4f92916a1b97d1c5a83cdf24671844306b2e3":
		event := new(TakerBidEvent)
		err = parsedAbi.UnpackIntoInterface(event, "TakerBid", data)
		if err != nil {
			panic(err)
		}
		fmt.Println("price", event.FeeAmounts[0].String())
		fmt.Println("fee", event.FeeAmounts[1].String())
	case "0x9aaa45d6db2ef74ead0751ea9113263d1dec1b50cea05f0ca2002cb8063564a4":
		event := new(TakerAskEvent)
		err = parsedAbi.UnpackIntoInterface(event, "TakerAsk", data)
		if err != nil {
			panic(err)
		}
		fmt.Println("price", event.FeeAmounts[0].String())
		fmt.Println("fee", event.FeeAmounts[1].String())
	}

}
