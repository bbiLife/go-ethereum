package filters

import (
	"context"
	"encoding/hex"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"strconv"
)

func (api *PublicFilterAPI) NewPendingSwapEnableTransactions(ctx context.Context, target common.Address) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		txHashes := make(chan []common.Hash, 128)
		pendingTxSub := api.events.SubscribePendingTxs(txHashes)
		log.Info("SubscribePendingTxs", "target", target)
		for {
			select {
			case hashes := <-txHashes:
				// To keep the original behaviour, send a single tx hash in one notification.
				// TODO(rjl493456442) Send a batch of tx hashes in one notification
				for _, h := range hashes {
					tx, _, err := api.backend.TransactionByHash(ctx, h)
					if err != nil {
						log.Error("Err", "Subscription", "err", err)
						continue
					}
					if !isSwapEnable(h, tx.Data(), target) {
						continue
					}
					notifier.Notify(rpcSub.ID, h)
				}
			case <-rpcSub.Err():
				pendingTxSub.Unsubscribe()
				return
			case <-notifier.Closed():
				pendingTxSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

func isSwapEnable(txHash common.Hash, txData []byte, target common.Address) bool {
	if len(txData) < 4 {
		return false
	}
	methodHex := hex.EncodeToString(txData[:4])
	switch methodHex {
	case "51d48cea":
		para := txData[4:]
		to := para[:32]
		if common.BytesToAddress(to) != target {
			return false
		}
		val := para[32:]
		v, _ := strconv.ParseInt(hex.EncodeToString(val), 10, 64)
		return v != 0
	case "e01af92c":
		v, _ := strconv.ParseInt(hex.EncodeToString(txData[4:]), 10, 64)
		return v != 0
	case "6a761202":
		para := txData[4:]
		to := para[:32]
		if common.BytesToAddress(to) != target {
			return false
		}
		para = para[32:]
		para = para[320:]
		data := para[:36]
		return isSwapEnable(txHash, data, target)
	default:
		log.Info("check tx", "tx", txData, "method", methodHex)
	}
	return false
}
