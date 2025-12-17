package contract

import (
	"context"
	"errors"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"uttc-hack-back-onchain/model"
)

// ContractGateway はスマートコントラクトとの連携を担当
type ContractGateway interface {
	// GetItem はコントラクトから商品情報を取得
	GetItem(ctx context.Context, itemId uint64) (*model.ContractItem, error)

	// SubscribeEvents はコントラクトイベントを購読
	SubscribeEvents(ctx context.Context) (<-chan *model.ContractEvent, error)

	// ScanPastEvents は過去のブロックからイベントをスキャン
	ScanPastEvents(ctx context.Context, fromBlock uint64, toBlock *uint64) (<-chan *model.ContractEvent, error)

	// GetContractAddress はコントラクトアドレスを返す
	GetContractAddress() string

	// VerifyTransaction はトランザクションを検証
	VerifyTransaction(ctx context.Context, txHash string) (*model.TxVerification, error)
}

// FrimaContractGateway はFrimaMarketplaceコントラクトとの連携実装
type FrimaContractGateway struct {
	client          *ethclient.Client
	contractAddress common.Address
	contractABI     abi.ABI
}

// NewFrimaContractGateway は新しいコントラクトゲートウェイを作成
func NewFrimaContractGateway(client *ethclient.Client, contractAddr string) (*FrimaContractGateway, error) {
	parsedABI, err := abi.JSON(strings.NewReader(FrimaMarketplaceABI))
	if err != nil {
		log.Printf("Failed to parse ABI: %v", err)
		return nil, err
	}

	contractAddress := common.HexToAddress(contractAddr)
	log.Printf("Initializing contract gateway for address: %s", contractAddress.Hex())
	
	// イベントが正しく定義されているか確認
	if _, ok := parsedABI.Events["ItemListed"]; !ok {
		log.Printf("WARNING: ItemListed event not found in ABI")
	} else {
		log.Printf("ItemListed event signature: %s", parsedABI.Events["ItemListed"].ID.Hex())
	}

	return &FrimaContractGateway{
		client:          client,
		contractAddress: contractAddress,
		contractABI:     parsedABI,
	}, nil
}

func (g *FrimaContractGateway) GetContractAddress() string {
	return g.contractAddress.Hex()
}

// GetLatestBlock は最新のブロックヘッダーを取得
func (g *FrimaContractGateway) GetLatestBlock(ctx context.Context) (*types.Header, error) {
	return g.client.HeaderByNumber(ctx, nil)
}

// GetItem はコントラクトから商品情報を取得
func (g *FrimaContractGateway) GetItem(ctx context.Context, itemId uint64) (*model.ContractItem, error) {
	data, err := g.contractABI.Pack("getItem", big.NewInt(int64(itemId)))
	if err != nil {
		return nil, err
	}

	msg := ethereum.CallMsg{
		To:   &g.contractAddress,
		Data: data,
	}

	result, err := g.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, err
	}

	// 結果をデコード
	var item struct {
		ItemId      *big.Int
		TokenId     *big.Int
		Title       string
		Price       *big.Int
		Explanation string
		ImageUrl    string
		Uid         string
		CreatedAt   *big.Int
		UpdatedAt   *big.Int
		IsPurchased bool
		Category    string
		Seller      common.Address
		Buyer       common.Address
		Status      uint8
	}

	err = g.contractABI.UnpackIntoInterface(&item, "getItem", result)
	if err != nil {
		return nil, err
	}

	return &model.ContractItem{
		ItemId:      item.ItemId.Uint64(),
		TokenId:     item.TokenId.Uint64(),
		Title:       item.Title,
		Price:       item.Price,
		Explanation: item.Explanation,
		ImageUrl:    item.ImageUrl,
		Uid:         item.Uid,
		CreatedAt:   item.CreatedAt.Uint64(),
		UpdatedAt:   item.UpdatedAt.Uint64(),
		IsPurchased: item.IsPurchased,
		Category:    item.Category,
		Seller:      item.Seller.Hex(),
		Buyer:       item.Buyer.Hex(),
		Status:      item.Status,
	}, nil
}

// SubscribeEvents はコントラクトイベントをWebSocket経由で購読
func (g *FrimaContractGateway) SubscribeEvents(ctx context.Context) (<-chan *model.ContractEvent, error) {
	eventChan := make(chan *model.ContractEvent, 100)

	query := ethereum.FilterQuery{
		Addresses: []common.Address{g.contractAddress},
	}

	log.Printf("Subscribing to events for contract: %s", g.contractAddress.Hex())
	logs := make(chan types.Log)
	sub, err := g.client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		log.Printf("Failed to subscribe to events: %v", err)
		return nil, err
	}
	log.Printf("Successfully subscribed to events")

	go func() {
		defer close(eventChan)
		defer sub.Unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return
			case err := <-sub.Err():
				log.Printf("Event subscription error: %v", err)
				return
			case vLog := <-logs:
				log.Printf("Received log from subscription (tx: %s, block: %d, topics: %d)", vLog.TxHash.Hex(), vLog.BlockNumber, len(vLog.Topics))
				event := g.parseLog(vLog)
				if event != nil {
					log.Printf("Successfully parsed event: %s for item %d", event.Type, event.ItemId)
					eventChan <- event
				} else {
					log.Printf("Failed to parse event from log (tx: %s)", vLog.TxHash.Hex())
				}
			}
		}
	}()

	return eventChan, nil
}

// ScanPastEvents は過去のブロックからイベントをスキャン
func (g *FrimaContractGateway) ScanPastEvents(ctx context.Context, fromBlock uint64, toBlock *uint64) (<-chan *model.ContractEvent, error) {
	eventChan := make(chan *model.ContractEvent, 100)

	go func() {
		defer close(eventChan)

		// 現在のブロック番号を取得
		header, err := g.client.HeaderByNumber(ctx, nil)
		if err != nil {
			log.Printf("Failed to get latest block: %v", err)
			return
		}

		currentBlock := header.Number.Uint64()
		
		// fromBlockが0の場合は、最後の1000ブロックからスキャン
		actualFromBlock := fromBlock
		if fromBlock == 0 {
			if currentBlock > 1000 {
				actualFromBlock = currentBlock - 1000
			}
		}

		actualToBlock := currentBlock
		if toBlock != nil {
			actualToBlock = *toBlock
		}

		query := ethereum.FilterQuery{
			Addresses: []common.Address{g.contractAddress},
			FromBlock: new(big.Int).SetUint64(actualFromBlock),
			ToBlock:   new(big.Int).SetUint64(actualToBlock),
		}

		log.Printf("Scanning past events from block %d to %d", actualFromBlock, actualToBlock)

		logs, err := g.client.FilterLogs(ctx, query)
		if err != nil {
			log.Printf("Failed to scan past events: %v", err)
			return
		}

		log.Printf("Found %d past events", len(logs))

		for i, vLog := range logs {
			log.Printf("Processing log %d/%d (tx: %s, block: %d)", i+1, len(logs), vLog.TxHash.Hex(), vLog.BlockNumber)
			event := g.parseLog(vLog)
			if event != nil {
				log.Printf("Successfully parsed past event: %s for item %d", event.Type, event.ItemId)
				eventChan <- event
			} else {
				log.Printf("Failed to parse past event from log (tx: %s)", vLog.TxHash.Hex())
			}
		}

		log.Printf("Finished scanning past events")
	}()

	return eventChan, nil
}

// parseLog はログをContractEventに変換
func (g *FrimaContractGateway) parseLog(vLog types.Log) *model.ContractEvent {
	if len(vLog.Topics) == 0 {
		log.Printf("Received log with no topics (tx: %s)", vLog.TxHash.Hex())
		return nil
	}

	eventSig := vLog.Topics[0].Hex()
	
	// デバッグ用: イベントシグネチャをログに出力
	log.Printf("Parsing log with event signature: %s (tx: %s, block: %d)", eventSig, vLog.TxHash.Hex(), vLog.BlockNumber)

	// 各イベントのシグネチャを確認
	itemListedSig := g.contractABI.Events["ItemListed"].ID.Hex()
	itemPurchasedSig := g.contractABI.Events["ItemPurchased"].ID.Hex()
	itemUpdatedSig := g.contractABI.Events["ItemUpdated"].ID.Hex()
	itemCancelledSig := g.contractABI.Events["ItemCancelled"].ID.Hex()
	receiptConfirmedSig := g.contractABI.Events["ReceiptConfirmed"].ID.Hex()

	log.Printf("Event signatures - ItemListed: %s, ItemPurchased: %s", itemListedSig, itemPurchasedSig)

	switch eventSig {
	case itemListedSig:
		log.Printf("Matched ItemListed event")
		return g.parseItemListed(vLog)
	case itemPurchasedSig:
		log.Printf("Matched ItemPurchased event")
		return g.parseItemPurchased(vLog)
	case itemUpdatedSig:
		log.Printf("Matched ItemUpdated event")
		return g.parseItemUpdated(vLog)
	case itemCancelledSig:
		log.Printf("Matched ItemCancelled event")
		return g.parseItemCancelled(vLog)
	case receiptConfirmedSig:
		log.Printf("Matched ReceiptConfirmed event")
		return g.parseReceiptConfirmed(vLog)
	default:
		log.Printf("Unknown event signature: %s (expected one of: ItemListed=%s, ItemPurchased=%s, etc.)", eventSig, itemListedSig, itemPurchasedSig)
		return nil
	}
}

func (g *FrimaContractGateway) parseItemListed(vLog types.Log) *model.ContractEvent {
	event := &model.ContractEvent{
		Type:    model.EventItemListed,
		TxHash:  vLog.TxHash.Hex(),
		BlockNo: vLog.BlockNumber,
	}

	// indexed: itemId, tokenId, seller
	if len(vLog.Topics) >= 4 {
		event.ItemId = new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Uint64()
		event.TokenId = new(big.Int).SetBytes(vLog.Topics[2].Bytes()).Uint64()
		event.Seller = common.HexToAddress(vLog.Topics[3].Hex()).Hex()
	}

	// non-indexed データをデコード
	data := make(map[string]interface{})
	err := g.contractABI.UnpackIntoMap(data, "ItemListed", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack ItemListed: %v", err)
		return event
	}

	if title, ok := data["title"].(string); ok {
		event.Title = title
	}
	if price, ok := data["price"].(*big.Int); ok {
		event.Price = price
	}
	if explanation, ok := data["explanation"].(string); ok {
		event.Explanation = explanation
	}
	if imageUrl, ok := data["imageUrl"].(string); ok {
		event.ImageUrl = imageUrl
	}
	if uid, ok := data["uid"].(string); ok {
		event.Uid = uid
		log.Printf("Parsed uid from ItemListed event: %s", uid)
	} else {
		log.Printf("WARNING: uid not found in ItemListed event data")
	}
	if createdAt, ok := data["createdAt"].(*big.Int); ok {
		event.CreatedAt = createdAt.Uint64()
	}
	if category, ok := data["category"].(string); ok {
		event.Category = category
	}

	log.Printf("Parsed ItemListed event: itemId=%d, title=%s, uid=%s, seller=%s, price=%s", event.ItemId, event.Title, event.Uid, event.Seller, event.Price.String())
	return event
}

func (g *FrimaContractGateway) parseItemPurchased(vLog types.Log) *model.ContractEvent {
	event := &model.ContractEvent{
		Type:    model.EventItemPurchased,
		TxHash:  vLog.TxHash.Hex(),
		BlockNo: vLog.BlockNumber,
	}

	// indexed: itemId, buyer
	if len(vLog.Topics) >= 3 {
		event.ItemId = new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Uint64()
		event.Buyer = common.HexToAddress(vLog.Topics[2].Hex()).Hex()
	}

	// non-indexed データをデコード
	data := make(map[string]interface{})
	err := g.contractABI.UnpackIntoMap(data, "ItemPurchased", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack ItemPurchased: %v", err)
		return event
	}

	if price, ok := data["price"].(*big.Int); ok {
		event.Price = price
	}
	if tokenId, ok := data["tokenId"].(*big.Int); ok {
		event.TokenId = tokenId.Uint64()
	}

	return event
}

func (g *FrimaContractGateway) parseItemUpdated(vLog types.Log) *model.ContractEvent {
	event := &model.ContractEvent{
		Type:    model.EventItemUpdated,
		TxHash:  vLog.TxHash.Hex(),
		BlockNo: vLog.BlockNumber,
	}

	// indexed: itemId
	if len(vLog.Topics) >= 2 {
		event.ItemId = new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Uint64()
	}

	// non-indexed データをデコード
	data := make(map[string]interface{})
	err := g.contractABI.UnpackIntoMap(data, "ItemUpdated", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack ItemUpdated: %v", err)
		return event
	}

	if title, ok := data["title"].(string); ok {
		event.Title = title
	}
	if price, ok := data["price"].(*big.Int); ok {
		event.Price = price
	}
	if explanation, ok := data["explanation"].(string); ok {
		event.Explanation = explanation
	}
	if imageUrl, ok := data["imageUrl"].(string); ok {
		event.ImageUrl = imageUrl
	}
	if category, ok := data["category"].(string); ok {
		event.Category = category
	}
	if updatedAt, ok := data["updatedAt"].(*big.Int); ok {
		event.UpdatedAt = updatedAt.Uint64()
	}

	return event
}

func (g *FrimaContractGateway) parseItemCancelled(vLog types.Log) *model.ContractEvent {
	event := &model.ContractEvent{
		Type:    model.EventItemCancelled,
		TxHash:  vLog.TxHash.Hex(),
		BlockNo: vLog.BlockNumber,
	}

	// indexed: itemId, seller
	if len(vLog.Topics) >= 3 {
		event.ItemId = new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Uint64()
		event.Seller = common.HexToAddress(vLog.Topics[2].Hex()).Hex()
	}

	return event
}

func (g *FrimaContractGateway) parseReceiptConfirmed(vLog types.Log) *model.ContractEvent {
	event := &model.ContractEvent{
		Type:    model.EventReceiptConfirmed,
		TxHash:  vLog.TxHash.Hex(),
		BlockNo: vLog.BlockNumber,
	}

	// indexed: itemId, buyer, seller
	if len(vLog.Topics) >= 4 {
		event.ItemId = new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Uint64()
		event.Buyer = common.HexToAddress(vLog.Topics[2].Hex()).Hex()
		event.Seller = common.HexToAddress(vLog.Topics[3].Hex()).Hex()
	}

	// non-indexed データをデコード
	data := make(map[string]interface{})
	err := g.contractABI.UnpackIntoMap(data, "ReceiptConfirmed", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack ReceiptConfirmed: %v", err)
		return event
	}

	if price, ok := data["price"].(*big.Int); ok {
		event.Price = price
	}

	return event
}

// VerifyTransaction はトランザクションを検証
func (g *FrimaContractGateway) VerifyTransaction(ctx context.Context, txHash string) (*model.TxVerification, error) {
	txHashObj := common.HexToHash(txHash)
	if txHashObj.Big().Cmp(big.NewInt(0)) == 0 {
		return nil, errors.New("invalid transaction hash format")
	}

	tx, isPending, err := g.client.TransactionByHash(ctx, txHashObj)
	if err != nil {
		return nil, errors.New("transaction not found")
	}

	if isPending {
		return &model.TxVerification{
			TxHash:  txHash,
			Status:  "pending",
			Success: false,
		}, nil
	}

	receipt, err := g.client.TransactionReceipt(ctx, txHashObj)
	if err != nil {
		return nil, errors.New("failed to get transaction receipt")
	}

	verification := &model.TxVerification{
		TxHash:      txHash,
		BlockNumber: receipt.BlockNumber.Uint64(),
		GasUsed:     receipt.GasUsed,
		Success:     receipt.Status == types.ReceiptStatusSuccessful,
	}

	if receipt.Status == types.ReceiptStatusSuccessful {
		verification.Status = "success"
	} else {
		verification.Status = "failed"
	}

	// コントラクト呼び出しかどうかを確認
	if tx.To() != nil && *tx.To() == g.contractAddress {
		verification.IsContractCall = true
	}

	return verification, nil
}
