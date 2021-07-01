package main

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"
	"sort"

	log "github.com/inconshreveable/log15"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/internal/ethapi"
)

//func generateTestContracts() []ethereum.CallMsg {
//	testMessages := make([]ethereum.CallMsg, 3)
//
//	addr1 := common.HexToAddress("0x7b73644935b8e68019ac6356c40661e1bc315860")
//	testMessages[0] = ethereum.CallMsg{
//		To:   &addr1,
//		Data: common.FromHex("0x0902f1ac"),
//	}
//
//	addr2 := common.HexToAddress("0x7a250d5630b4cf539739df2c5dacb4c659f2488d")
//	testMessages[1] = ethereum.CallMsg{
//		To:    &addr2,
//		Data:  common.FromHex("0x7ff36ab500000000000000000000000000000000000000000288598ee8cc37a82aed4c340000000000000000000000000000000000000000000000000000000000000080000000000000000000000000741a704647fb24da9988be03219b55e485704fc5000000000000000000000000000000000000000000000000000000007099cf9a0000000000000000000000000000000000000000000000000000000000000002000000000000000000000000c02aaa39b223fe8d0a0e5c4f27ead9083c756cc2000000000000000000000000761d38e5ddf6ccf6cf7c55759d5210750b5d60f3"),
//		Value: big.NewInt(0x186cc6acd4b0000),
//	}
//
//	testMessages[2] = ethereum.CallMsg{
//		To:   &addr1,
//		Data: common.FromHex("0x0902f1ac"),
//	}
//
//	return testMessages
//}

func getSortedTxpoolContent(client *ethclient.Client) []*ethapi.RPCTransaction {
	res1, err1 := client.GetTxpoolContent(context.Background())
	if err1 != nil {
		fmt.Println(err1)
	}

	newTransactions := res1["queued"]

	flattened := make([]*ethapi.RPCTransaction, len(newTransactions))
	var counter int = 0
	for _, v := range newTransactions {
		var maxNonce hexutil.Uint64 = hexutil.Uint64(0)
		// TODO: This logic is incorrect
		for _, transaction := range v {
			if transaction.Nonce > maxNonce {
				flattened[counter] = transaction
				maxNonce = transaction.Nonce
			}
		}
		counter++
	}
	sort.Slice(flattened, func(i, j int) bool {
		return flattened[i].GasPrice.ToInt().Uint64() > flattened[j].GasPrice.ToInt().Uint64()
	})
	return flattened
}

func toCallMsgs(transactions []*ethapi.RPCTransaction) []ethereum.CallMsg {
	msgs := make([]ethereum.CallMsg, len(transactions))
	for idx, transaction := range transactions {
		msgs[idx] = ethereum.CallMsg{
			From:     transaction.From,
			To:       transaction.To,
			Gas:      uint64(transaction.Gas),
			GasPrice: transaction.GasPrice.ToInt(),
			Value:    transaction.Value.ToInt(),
			Data:     transaction.Input,
		}
	}
	return msgs
}

type Reserve struct {
	Address common.Address
	Token0  *big.Int
	Token1  *big.Int
}

func processLogs(msgs []ethereum.CallMsg, poolLogs [][]ethapi.LogResult) {
	syncString := "0x1c411e9a96e071241c2f21f7726b17ae89e3cab4c78be50e062b03a9fffbbad1"
	// FIXME: gas price could hypothetically be bigger?
	// var gasPriceToReserves map[uint64][]Reserve
	gasPriceToReserves := make(map[uint64][]Reserve)
	for idx, transactionLogs := range poolLogs {
		msg := msgs[idx]
		gasPrice := msg.GasPrice
		for _, log := range transactionLogs {
			anySyncs := false
			for _, topic := range log.Topics {
				if topic.String() == syncString {
					anySyncs = true
				}
			}
			if anySyncs {
				if len(log.Data) != 64 {
					panic("Length is not 64!")
				}
				var token0, token1 big.Int
				token0.SetBytes(log.Data[:32])
				token1.SetBytes(log.Data[32:])
				fmt.Println(gasPrice, log.Address, log.Data)
				gasPriceToReserves[gasPrice.Uint64()] = append(gasPriceToReserves[gasPrice.Uint64()], Reserve{Address: log.Address, Token0: &token0, Token1: &token1})
			}
		}
	}
}

func main() {
	var ipcPath string
	flag.StringVar(&ipcPath, "ipc", "/home/daniel/.ethereum/geth.ipc", "")
	flag.Parse()

	client, err := ethclient.Dial(ipcPath)
	if err != nil {
		log.Error("Exiting", "err", err.Error())
		os.Exit(1)
	}
	defer client.Close()

	/*callMsgs := generateTestContracts()
	res, err := client.CallContracts(context.Background(), callMsgs, nil)
	if err != nil {
		// log.Fatalf("Error calling contract: %v", err)
	}
	fmt.Println(res)*/

	msgs := toCallMsgs(getSortedTxpoolContent(client))
	res, err := client.CallContracts(context.Background(), msgs, nil)
	/* for idx, logs := range res.LogRes {
		fmt.Println(idx, logs)
	}
	fmt.Println(len(res.LogRes))*/

	processLogs(msgs, res.LogRes)
	/*res1, err1 := client.GetTxpoolContent(context.Background())
	if err1 != nil {

	}
	fmt.Println(res1)*/
}
