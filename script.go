package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	aabi "github.com/inscription/abi"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"math/big"
	"sort"
	"strings"
	"time"
)

var missParseEventError = errors.New("not found event")

const createContractBlock = 18856232
const endSearchBlock = 18910071
const step = 500
const rpcUrl = "https://eth-mainnet.g.alchemy.com/v2/iqxe7N1WGS9PAtAjyWXalovrFIY-T114"

const contract = "0x8c578a6e31fc94b1facd58202be53a8385bacbf7"
const trimLeft = "0x000000000000000000000000"

var myABI *abi.ABI
var abiCaller *aabi.Erc721gpCaller
var myDB *sql.DB
var recordBlock int64

var orderFulfilled = "0x9d9af8e38d66c62e2c12f0225249fd9d721c54b83f48d9352c97c6cacdcb6f31"
var eRC721BuyOrderFilled = crypto.Keccak256Hash([]byte(`ERC721BuyOrderFilled (bytes32 orderHash, address maker, address taker, uint256 nonce, address erc20Token, uint256 erc20TokenAmount, tuple[] fees, address erc721Token, uint256 erc721TokenId)`)).Hex()
var eRC721SellOrderFilled = "0x9c248aa1a265aa616f707b979d57f4529bb63a4fc34dc7fc61fdddc18410f74e"
var takerBid = "0x3ee3de4684413690dee6fff1a0a4f92916a1b97d1c5a83cdf24671844306b2e3"
var takerAsk = "0x9aaa45d6db2ef74ead0751ea9113263d1dec1b50cea05f0ca2002cb8063564a4"
var execution = crypto.Keccak256Hash([]byte(`Execution(Transfer transfer, bytes32 orderHash, uint256 listingIndex, uint256 price, FeeRate makerFee, Fees fees, OrderType orderType)`)).Hex()
var execution721Taker = crypto.Keccak256Hash([]byte(`Execution721TakerFeePacked(bytes32,uint256,uint256,uint256)`)).Hex()
var execution721 = crypto.Keccak256Hash([]byte(`Execution721Packed(bytes32,uint256,uint256)`)).Hex()
var execution721Maker = crypto.Keccak256Hash([]byte(`Execution721MakerFeePacked(bytes32,uint256,uint256,uint256)`)).Hex()

func main() {
	db, err := sql.Open("sqlite3", "./fraud.sqlite")
	if err != nil {
		panic(err)
	}
	myDB = db
	defer db.Close()

	a, err := aabi.Erc721gpMetaData.GetAbi()
	if err != nil {
		panic(err)
	}

	myABI = a

	c, e := ethclient.Dial(rpcUrl)
	if e != nil {
		panic(e)
	}
	rc, e := rpc.Dial(rpcUrl)
	if e != nil {
		panic(e)
	}

	caller, e := aabi.NewErc721gpCaller(common.HexToAddress(contract), c)
	if e != nil {
		panic(e)
	}

	abiCaller = caller

	rows, err := myDB.Query("SELECT number FROM number")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var start int64

	for rows.Next() {
		if err := rows.Scan(&start); err != nil {
			panic(err)
		}
	}

	recordBlock = start

	if _, err := myDB.Exec("DELETE FROM transfers WHERE block_number >= ?", start); err != nil {
		panic(err)
	}

	bn := uint64(endSearchBlock)
	end := int64(bn)

	for start < end {
		if end > start+step {
			end = start + step
		}
		findAB(c, rc, big.NewInt(start), big.NewInt(end))
		start = end + 1
		end = int64(bn)
	}
}

func findAB(c *ethclient.Client, rc *rpc.Client, from, to *big.Int) {
	q := ethereum.FilterQuery{
		FromBlock: from,
		ToBlock:   to,
		Addresses: []common.Address{common.HexToAddress(contract)},
		Topics: [][]common.Hash{
			{common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")},
		},
	}
	logs, e := c.FilterLogs(context.Background(), q)
	if e != nil {
		panic(e)
	}

	transactions := make(map[string]*types.Transaction)
	tracesStore := make(map[string][]WrapperAction)
	receipts := make(map[common.Hash]*types.Receipt)
	for _, l := range logs {
		if int64(l.BlockNumber) > recordBlock {
			if _, err := myDB.Exec("UPDATE number SET number = ?", l.BlockNumber); err != nil {
				panic(err)
			}
			recordBlock = int64(l.BlockNumber)
		}
		fa := "0x" + strings.TrimPrefix(l.Topics[1].Hex(), trimLeft)
		if fa == "0x0000000000000000000000000000000000000000" {
			continue
		}
		ta := "0x" + strings.TrimPrefix(l.Topics[2].Hex(), trimLeft)
		tokenId, ok := new(big.Int).SetString(l.Topics[3].Hex(), 0)
		if !ok {
			panic("wrong token id " + l.TxHash.Hex())
		}

		if _, err := myDB.Exec(`INSERT INTO transfers (hash,block_number,from_address,to_address,tokenId) VALUES (?,?,?,?,?)`, l.TxHash.Hex(), l.BlockNumber, fa, ta, tokenId.Int64()); err != nil {
			panic(err)
		}
		fmt.Println("handle token ", tokenId.Int64(), "hash ", l.TxHash.Hex(), "block number ", l.BlockNumber)

		time.Sleep(10 * time.Millisecond)

		if _, found := transactions[l.TxHash.Hex()]; !found {
			tx, _, err := c.TransactionByHash(context.Background(), l.TxHash)
			if err != nil {
				panic(err)
			}
			transactions[l.TxHash.Hex()] = tx
		}

		tx := transactions[l.TxHash.Hex()]

		sender, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err != nil {
			panic(err)
		}

		owner := getOwner(tokenId, big.NewInt(int64(l.BlockNumber-1)))

		// 1
		if strings.ToLower(sender.Hex()) == strings.ToLower(owner) {
			continue
		}

		// 3 - trace
		if _, found := tracesStore[tx.Hash().Hex()]; !found {
			tracesStore[tx.Hash().Hex()] = getTrace(rc, tx.Hash())
		}

		// 3.1 is from exchange market
		var exchangeMarket string

		traces := tracesStore[tx.Hash().Hex()]

		for i := range traces {
			if strings.ToLower(traces[i].Action.To) != strings.ToLower(contract) {
				continue
			}
			var tid int64 = -1
			if strings.HasPrefix(traces[i].Action.Input, "0x23b872dd") {
				inputs, err := myABI.Methods["transferFrom"].Inputs.Unpack(hexutil.MustDecode("0x" + strings.TrimPrefix(traces[i].Action.Input, "0x23b872dd")))
				if err != nil {
					panic(err)
				}

				id, ok := inputs[2].(*big.Int)
				if !ok {
					panic(inputs)
				}

				tid = id.Int64()
			}
			if strings.HasPrefix(traces[i].Action.Input, "0x42842e0e") {
				inputs, err := myABI.Methods["safeTransferFrom"].Inputs.UnpackValues(hexutil.MustDecode("0x" + strings.TrimPrefix(traces[i].Action.Input, "0x42842e0e")))
				if err != nil {
					panic(err)
				}
				id, ok := inputs[2].(*big.Int)
				if !ok {
					panic(inputs)
				}
				tid = id.Int64()
			}
			if strings.HasPrefix(traces[i].Action.Input, "0xb88d4fde") {
				inputs, err := myABI.Methods["safeTransferFrom"].Inputs.UnpackValues(hexutil.MustDecode("0x" + strings.TrimPrefix(traces[i].Action.Input, "0xb88d4fde")))
				if err != nil {
					panic(err)
				}
				id, ok := inputs[2].(*big.Int)
				if !ok {
					panic(inputs)
				}
				tid = id.Int64()
			}

			if tid != tokenId.Int64() {
				continue
			}

			if strings.ToLower(traces[i].Action.From) == strings.ToLower(owner) {
				break
			}

			exchangeMarket = isExchange(traces[i].Action.From)
			if exchangeMarket != "" {
				break
			} else {
				// 3.2
				isOperator, err := abiCaller.IsApprovedForAll(&bind.CallOpts{
					BlockNumber: big.NewInt(int64(l.BlockNumber - 1)),
				}, common.HexToAddress(owner), common.HexToAddress(traces[i].Action.From))
				if err != nil {
					panic(err)
				}
				if isOperator {
					break
				}

				// 3.3 latest approve owner
				t := int64(l.BlockNumber)
				s := t - step
				if s < createContractBlock {
					s = createContractBlock
				}

				for {
					approveHash, ownerAddress := getLatestApprove(c, big.NewInt(s), big.NewInt(t), tokenId, l.TxHash.Hex())
					var trigger string
					if approveHash != "" {
						if _, found := tracesStore[approveHash]; !found {
							tracesStore[approveHash] = getTrace(rc, common.HexToHash(approveHash))
						}
						newTraces := tracesStore[approveHash]
						for i := range newTraces {
							if strings.ToLower(newTraces[i].Action.To) != strings.ToLower(contract) {
								continue
							}
							if strings.HasPrefix(newTraces[i].Action.Input, "0x095ea7b3") {
								inputs, err := myABI.Methods["approve"].Inputs.UnpackValues(hexutil.MustDecode("0x" + strings.TrimPrefix(newTraces[i].Action.Input, "0x095ea7b3")))
								if err != nil {
									panic(err)
								}

								id, ok := inputs[1].(*big.Int)
								if !ok {
									panic(inputs)
								}
								if id.Int64() != tokenId.Int64() {
									continue
								}

								trigger = newTraces[i].Action.From
								break
							}
						}

						if strings.ToLower(trigger) != strings.ToLower(ownerAddress) {
							if _, err := myDB.Exec("UPDATE transfers SET approveHash = ?,isFraud = ?,fraudSender = ?,fraudReceiver = ? WHERE hash = ? AND tokenId = ?", approveHash, true, sender, to, l.TxHash.Hex(), tokenId.Int64()); err != nil {
								panic(err)
							}
							fmt.Printf("fraud transaction %s,block number %d, from %s to %s token %d, approve hash %s\n", l.TxHash.Hex(), l.BlockNumber, fa, ta, tokenId, approveHash)
						}
						break
					}
					t = s - 1
					if t <= createContractBlock {
						break
					}
					s = t - step
					if s < createContractBlock {
						s = createContractBlock
					}
				}

				if s == createContractBlock {
					if _, err := myDB.Exec("UPDATE transfers SET isFraud = ? WHERE hash = ? AND tokenId = ?", true, l.TxHash.Hex(), tokenId.Int64()); err != nil {
						panic(err)
					}
					fmt.Printf("not found approved address transaction %s, token %d", l.TxHash.Hex(), tokenId)
				}
			}
		}

		if exchangeMarket != "" {
			// get price
			if _, found := receipts[tx.Hash()]; !found {
				rt, err := c.TransactionReceipt(context.Background(), tx.Hash())
				if err != nil {
					panic(err)
				}
				receipts[tx.Hash()] = rt
			}

			switch exchangeMarket {
			case "element":
				price, fee, e := findPriceElement(receipts[tx.Hash()], tokenId.Int64())
				if e != nil {
					if _, e1 := myDB.Exec("UPDATE transfers SET  market = 'element',err = ? WHERE hash = ? AND tokenId = ?", e.Error(), l.TxHash.Hex(), tokenId.Int64()); e1 != nil {
						panic(e1)
					}
				} else {
					if _, e1 := myDB.Exec("UPDATE transfers SET price = ?,fee = ?, market = 'element' WHERE hash = ? AND tokenId = ?", price.String(), fee.String(), l.TxHash.Hex(), tokenId.Int64()); e1 != nil {
						panic(e1)
					}
				}
			case "looksRare":
				price, fee, e := findPriceLR(receipts[tx.Hash()], tokenId.Int64())
				if e != nil {
					if _, e1 := myDB.Exec("UPDATE transfers SET  market = 'looksRare',err = ? WHERE hash = ? AND tokenId = ?", e.Error(), l.TxHash.Hex(), tokenId.Int64()); e1 != nil {
						panic(e1)
					}
				} else {
					if _, e1 := myDB.Exec("UPDATE transfers SET price = ?,fee = ?, market = 'looksRare' WHERE hash = ? AND tokenId = ?", price.String(), fee.String(), l.TxHash.Hex(), tokenId.Int64()); e1 != nil {
						panic(e1)
					}
				}
			case "openSea":
				price, fee, e := findPriceOpenSea(receipts[tx.Hash()], tokenId.Int64())
				if e != nil {
					if _, e1 := myDB.Exec("UPDATE transfers SET  market = 'openSea',err = ? WHERE hash = ? AND tokenId = ?", e.Error(), l.TxHash.Hex(), tokenId.Int64()); e1 != nil {
						panic(e1)
					}
				} else {
					if _, e1 := myDB.Exec("UPDATE transfers SET price = ?,fee = ?, market = 'openSea' WHERE hash = ? AND tokenId = ?", price.String(), fee.String(), l.TxHash.Hex(), tokenId.Int64()); e1 != nil {
						panic(e1)
					}
				}
			case "blur":
				price, fee, e := findPriceBlur(receipts[tx.Hash()], tokenId.Int64())
				if e != nil {
					if _, e1 := myDB.Exec("UPDATE transfers SET  market = 'blur',err = ? WHERE hash = ? AND tokenId = ?", e.Error(), l.TxHash.Hex(), tokenId.Int64()); e1 != nil {
						panic(e1)
					}
				} else {
					if _, e1 := myDB.Exec("UPDATE transfers SET price = ?,fee = ?, market = 'blur' WHERE hash = ? AND tokenId = ?", price.String(), fee.String(), l.TxHash.Hex(), tokenId.Int64()); e1 != nil {
						panic(e1)
					}
				}
			}

		}
	}

}

var exchanges = map[string]string{"element": "0x20f780a973856b93f63670377900c1d2a50a77c4", "looksRare": "0x0000000000e655fae4d56241588680f86e3b2377", "openSea": "0x00000000000000adc04c56bf30ac9d3c0aaf14dc", "blur": "0xb2ecfe4e4d61f8790bbb9de2d1259b9e2410cea5"}

func isExchange(address string) string {
	for k, v := range exchanges {
		if strings.ToLower(v) == strings.ToLower(address) {
			return k
		}
	}
	return ""
}

type WrapperAction struct {
	Action Action `json:"action"`
}

type Action struct {
	From  string `json:"from"`
	Input string `json:"input"`
	To    string `json:"to"`
}

var t = time.NewTicker(300 * time.Millisecond)

func getTrace(c *rpc.Client, hash common.Hash) []WrapperAction {

	<-t.C

	var result interface{}

	for {
		err := c.CallContext(context.Background(), &result, "trace_transaction", hash.Hex())
		if err != nil {
			fmt.Println("Error ", err.Error())
			time.Sleep(time.Second)
		}
		break
	}

	data1, err1 := json.Marshal(result)
	if err1 != nil {
		panic(err1)
	}

	actions := make([]WrapperAction, 0)
	if err := json.Unmarshal(data1, &actions); err != nil {
		panic(err)
	}

	return actions
}

func getLatestApprove(c *ethclient.Client, start, end, tokenId *big.Int, transferHash string) (string, string) {
	fmt.Println("getLatestApprove", tokenId, start, end)
	q := ethereum.FilterQuery{
		FromBlock: start,
		ToBlock:   end,
		Addresses: []common.Address{common.HexToAddress(contract)},
		Topics: [][]common.Hash{
			{common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925")},
		},
	}
	logs, e := c.FilterLogs(context.Background(), q)
	if e != nil {
		panic(e)
	}

	for i := len(logs) - 1; i >= 0; i-- {
		if strings.ToLower(logs[i].TxHash.Hex()) == strings.ToLower(transferHash) && logs[i].Topics[2].Hex() == "0x0000000000000000000000000000000000000000000000000000000000000000" {
			continue
		}
		tid, ok := new(big.Int).SetString(logs[i].Topics[3].Hex(), 0)
		if !ok {
			panic(logs[i].TxHash.Hex())
		}
		if tokenId.Int64() == tid.Int64() {
			return logs[i].TxHash.Hex(), "0x" + strings.TrimPrefix(logs[i].Topics[1].Hex(), trimLeft)
		}
	}

	return "", ""
}

func getOwner(tokenID, bn *big.Int) string {
	owner, err := abiCaller.OwnerOf(&bind.CallOpts{BlockNumber: bn}, tokenID)
	if err != nil {
		fmt.Println(tokenID.Uint64())
		panic(err)
	}
	return owner.Hex()
}

var blurAbi, openSeaAbi, elementAbi, looksRareAbi abi.ABI

func init() {
	var err error
	blur := `[ {
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
	blurAbi, err = abi.JSON(strings.NewReader(blur))
	if err != nil {
		panic(err)
	}

	openSea := `[{
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
  }]`
	openSeaAbi, err = abi.JSON(strings.NewReader(openSea))
	if err != nil {
		panic(err)
	}

	element := `[
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
	elementAbi, err = abi.JSON(strings.NewReader(element))
	if err != nil {
		panic(err)
	}

	looksRare := `[{
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
	looksRareAbi, err = abi.JSON(strings.NewReader(looksRare))
	if err != nil {
		panic(err)
	}

}

func findPriceBlur(rt *types.Receipt, tokenId int64) (price *big.Int, totalFee *big.Int, err error) {
	log.Println("find price blur ", tokenId)
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

	for _, l := range rt.Logs {
		switch l.Topics[0].Hex() {
		case execution721:
			event := new(Execution721Packed)
			if err = blurAbi.UnpackIntoInterface(event, "Execution721Packed", l.Data); err != nil {
				return
			}
			if blurParseTokenId(event.TokenIdListingIndexTrader).Int64() != tokenId {
				break
			}
			price = blurParsePrice(event.CollectionPriceSide)
			totalFee = big.NewInt(0)
		case execution:
			event := new(Execution)
			if err = blurAbi.UnpackIntoInterface(event, "Execution", l.Data); err != nil {
				return
			}
			if event.TransferData.Id.Int64() != tokenId {
				break
			}
			price = event.Price
			fee := event.FeesData.ProtocolFee.Rate + event.FeesData.TakerFee.Rate + event.MakerFee.Rate
			totalFee = big.NewInt(int64(fee))
		case execution721Maker:
			event := new(Execution721MakerFeePacked)
			if err = blurAbi.UnpackIntoInterface(event, "Execution721MakerFeePacked", l.Data); err != nil {
				return
			}
			if blurParseTokenId(event.TokenIdListingIndexTrader).Int64() != tokenId {
				break
			}
			price = blurParsePrice(event.CollectionPriceSide)
			totalFee = big.NewInt(0)
		case execution721Taker:
			event := new(Execution721TakerFeePacked)
			if err = blurAbi.UnpackIntoInterface(event, "Execution721TakerFeePacked", l.Data); err != nil {
				return
			}
			if blurParseTokenId(event.TokenIdListingIndexTrader).Int64() != tokenId {
				break
			}
			price = blurParsePrice(event.CollectionPriceSide)
			totalFee = big.NewInt(0)
		}
	}

	return
}

func blurParsePrice(collectionPriceSide *big.Int) *big.Int {
	mask88 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 88), big.NewInt(1))
	price := new(big.Int).Rsh(collectionPriceSide, 160)
	price.And(price, mask88)

	return price
}

func blurParseTokenId(tokenIdListingIndexTrader *big.Int) *big.Int {
	mask88 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 88), big.NewInt(1))

	// 提取orderType (最高8位)
	tokenId := new(big.Int).Rsh(tokenIdListingIndexTrader, 168)
	tokenId.And(tokenId, mask88)
	return tokenId
}

func findPriceOpenSea(rt *types.Receipt, tokenId int64) (price *big.Int, totalFee *big.Int, err error) {
	log.Println("findPriceOpenSea price ", tokenId)
	type OfferItem struct {
		ItemType             uint8
		Token                common.Address
		IdentifierOrCriteria *big.Int
		Amount               *big.Int
	}
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

	var fillType string
	orderFulfilledEvents := make(map[[32]byte]*OrderFulfilledEvent)
	for _, l := range rt.Logs {
		if l.Topics[0].Hex() == orderFulfilled {
			event := new(OrderFulfilledEvent)
			err = openSeaAbi.UnpackIntoInterface(event, "OrderFulfilled", l.Data)
			if err != nil {
				return
			}
			for _, o := range event.Offer {
				if o.Token.Cmp(common.HexToAddress(contract)) == 0 {
					if o.IdentifierOrCriteria.Int64() == tokenId {
						if fillType != "" {
							fillType = "match"
						} else {
							fillType = "offer"
						}
						orderFulfilledEvents[event.OrderHash] = event
					}
				}
			}
			for _, c := range event.Consideration {
				if c.Token.Cmp(common.HexToAddress(contract)) == 0 {
					if c.IdentifierOrCriteria.Int64() == tokenId {
						if fillType != "" {
							fillType = "match"
						} else {
							fillType = "consider"
						}
						orderFulfilledEvents[event.OrderHash] = event
					}
				}
			}
		}
	}

	prices := make(PriceList, 0)
	for _, event := range orderFulfilledEvents {
		for _, o := range event.Offer {
			// 2: nft token
			if o.ItemType != 2 {
				prices = append(prices, o.Amount)
			}
		}
		for _, c := range event.Consideration {
			// 2: nft token
			if c.ItemType != 2 {
				prices = append(prices, c.Amount)
			}
		}
	}

	sort.Sort(prices)

	if len(orderFulfilledEvents) == 2 {
		if len(prices) < 3 {
			err = fmt.Errorf("wierd opensea transaction")
			return
		}

		fee := new(big.Int)
		for i := 2; i < len(prices); i++ {
			fee.Add(fee, prices[i])
		}
		price = prices[0]
		totalFee = fee
	} else if len(orderFulfilledEvents) == 1 {
		if len(prices) < 2 {
			err = fmt.Errorf("wierd opensea transaction")
			return
		}
		switch fillType {
		case "offer":
			fee := new(big.Int)
			p := prices[0]
			for i := 1; i < len(prices); i++ {
				fee.Add(fee, prices[i])
				p.Add(p, prices[i])
			}
			totalFee = fee
			price = p
		case "consider":
			fmt.Println("check transaction ", rt.TxHash.Hex())
			price = prices[0]
			fee := new(big.Int)
			for i := 1; i < len(prices); i++ {
				fee.Add(fee, prices[i])
			}
			totalFee = fee
		default:
			panic("wierd transaction " + rt.TxHash.Hex())
		}
	} else {
		err = missParseEventError
	}

	return
}

type PriceList []*big.Int

func (pl PriceList) Len() int           { return len(pl) }
func (pl PriceList) Swap(i, j int)      { pl[i], pl[j] = pl[j], pl[i] }
func (pl PriceList) Less(i, j int) bool { return pl[i].Cmp(pl[j]) > 0 }

func findPriceElement(rt *types.Receipt, tokenId int64) (price *big.Int, totalFee *big.Int, err error) {
	log.Println("findPriceElement ", tokenId)

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
	var find bool
	for _, l := range rt.Logs {
		switch l.Topics[0].Hex() {
		case eRC721SellOrderFilled:
			err = elementAbi.UnpackIntoInterface(event, "ERC721SellOrderFilled", l.Data)
			if err != nil {
				return
			}
			if event.Erc721TokenId.Int64() == tokenId {
				find = true
				break
			}
		case eRC721BuyOrderFilled:
			err = elementAbi.UnpackIntoInterface(event, "ERC721BuyOrderFilled", l.Data)
			if err != nil {
				return
			}
			if event.Erc721TokenId.Int64() == tokenId {
				find = true
				break
			}
		}
	}

	if !find {
		err = missParseEventError
		return
	}

	price = event.Erc20TokenAmount
	totalFee = big.NewInt(0)
	for _, fee := range event.Fees {
		totalFee.Add(totalFee, fee.Amount)
	}

	return
}

func findPriceLR(rt *types.Receipt, tokenId int64) (price *big.Int, totalFee *big.Int, err error) {
	log.Println("findPriceLR Looks Rare", tokenId)
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

	for _, l := range rt.Logs {
		switch l.Topics[0].Hex() {
		case takerBid:
			event := new(TakerBidEvent)
			err = looksRareAbi.UnpackIntoInterface(event, "TakerBid", l.Data)
			if err != nil {
				return
			}
			if len(event.ItemIds) != 1 {
				err = fmt.Errorf("chek Looks Rare transaction items: %d", len(event.ItemIds))
				return
			}
			if event.ItemIds[0].Int64() == tokenId {
				price = event.FeeAmounts[0]
				totalFee = big.NewInt(0)
				for i, fee := range event.FeeAmounts {
					if i > 0 {
						totalFee.Add(totalFee, fee)
					}
				}
				return
			}
		case takerAsk:
			event := new(TakerAskEvent)
			err = looksRareAbi.UnpackIntoInterface(event, "TakerAsk", l.Data)
			if len(event.ItemIds) != 1 {
				err = fmt.Errorf("chek Looks Rare transaction items: %d", len(event.ItemIds))
				return
			}
			if event.ItemIds[0].Int64() == tokenId {
				price = event.FeeAmounts[0]
				totalFee = big.NewInt(0)
				for i, fee := range event.FeeAmounts {
					if i > 0 {
						totalFee.Add(totalFee, fee)
					}
				}
				return
			}
		}
	}

	err = missParseEventError
	return
}
