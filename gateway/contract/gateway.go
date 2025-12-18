package contract

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

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
	log.Printf("Initializing contract gateway: %s", contractAddress.Hex())

	// イベントが正しく定義されているか確認
	eventNames := []string{"ItemListed", "ItemPurchased", "ItemUpdated", "ItemCancelled", "ReceiptConfirmed"}
	for _, eventName := range eventNames {
		if _, ok := parsedABI.Events[eventName]; !ok {
			log.Printf("WARNING: Event '%s' not found in ABI", eventName)
		}
	}

	if contractAddress == common.HexToAddress("0x0") || contractAddress == common.HexToAddress("0x0000000000000000000000000000000000000000") {
		log.Printf("WARNING: Contract address appears to be zero address")
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
// WebSocket接続が失敗した場合、定期的なポーリングにフォールバックする
func (g *FrimaContractGateway) SubscribeEvents(ctx context.Context) (<-chan *model.ContractEvent, error) {
	eventChan := make(chan *model.ContractEvent, 100)

	// 接続のヘルスチェック（タイムアウト付き）
	healthCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	header, err := g.client.HeaderByNumber(healthCtx, nil)
	if err != nil {
		return nil, fmt.Errorf("connection test failed (connection may be lost): %w", err)
	}

	// WebSocket接続を試みる
	query := ethereum.FilterQuery{
		Addresses: []common.Address{g.contractAddress},
	}
	logs := make(chan types.Log)
	sub, err := g.client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		log.Printf("WARNING: WebSocket subscription failed, falling back to polling: %v", err)
		go g.pollEvents(ctx, eventChan, header.Number.Uint64())
		return eventChan, nil
	}

	log.Printf("Subscribed to events via WebSocket (latest block: %d)", header.Number.Uint64())

	go func() {
		defer func() {
			sub.Unsubscribe()
			// WebSocket接続が切れたときにチャネルを閉じる（useCase側で再接続が試みられる）
			close(eventChan)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case err := <-sub.Err():
				log.Printf("ERROR: WebSocket error, channel will be closed for reconnection: %v", err)
				// チャネルを閉じてuseCase側の再接続ロジックをトリガー
				return
			case vLog, ok := <-logs:
				if !ok {
					log.Printf("WARNING: WebSocket logs channel closed, reconnecting...")
					return
				}
				if vLog.Address != g.contractAddress {
					continue
				}
				event := g.parseLog(vLog)
				if event != nil {
					log.Printf("Event received: %s itemId=%d tx=%s", event.Type, event.ItemId, event.TxHash)
					select {
					case eventChan <- event:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return eventChan, nil
}

// pollEvents は定期的にブロックチェーンをポーリングしてイベントを取得
// チャネルをクローズしない（永続実行）
func (g *FrimaContractGateway) pollEvents(ctx context.Context, eventChan chan<- *model.ContractEvent, startBlock uint64) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ERROR: pollEvents panic: %v", r)
			// パニック時はチャネルを閉じてuseCase側で再接続を試みる
			close(eventChan)
		}
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastProcessedBlock := startBlock
	log.Printf("Starting event polling from block %d", lastProcessedBlock)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 接続のヘルスチェック
				header, err := g.client.HeaderByNumber(ctx, nil)
				if err != nil {
					log.Printf("ERROR: Failed to get latest block (connection may be lost): %v", err)
					// 接続エラーの場合、チャネルを閉じてuseCase側で再接続を試みる
					close(eventChan)
					return
				}

			currentBlock := header.Number.Uint64()
			if currentBlock <= lastProcessedBlock {
				continue
			}

			query := ethereum.FilterQuery{
				Addresses: []common.Address{g.contractAddress},
				FromBlock: new(big.Int).SetUint64(lastProcessedBlock + 1),
				ToBlock:   new(big.Int).SetUint64(currentBlock),
			}

			logs, err := g.client.FilterLogs(ctx, query)
			if err != nil {
				log.Printf("ERROR: Failed to filter logs: %v", err)
				lastProcessedBlock = currentBlock
				continue
			}

			if len(logs) > 0 {
				log.Printf("Found %d events (blocks %d-%d)", len(logs), lastProcessedBlock+1, currentBlock)
			}

			for _, vLog := range logs {
				if vLog.Address != g.contractAddress {
					continue
				}
				event := g.parseLog(vLog)
				if event != nil {
					log.Printf("Event received: %s itemId=%d tx=%s", event.Type, event.ItemId, event.TxHash)
					select {
					case eventChan <- event:
					case <-ctx.Done():
						return
					}
				}
			}

			lastProcessedBlock = currentBlock
		}
	}
}

// ScanPastEvents は過去のブロックからイベントをスキャン
func (g *FrimaContractGateway) ScanPastEvents(ctx context.Context, fromBlock uint64, toBlock *uint64) (<-chan *model.ContractEvent, error) {
	eventChan := make(chan *model.ContractEvent, 100)

	go func() {
		defer close(eventChan)

		header, err := g.client.HeaderByNumber(ctx, nil)
		if err != nil {
			log.Printf("ERROR: Failed to get latest block: %v", err)
			return
		}

		currentBlock := header.Number.Uint64()
		actualFromBlock := fromBlock
		if fromBlock == 0 {
			// より広い範囲をスキャン（10000ブロック、約1.4日分）
			if currentBlock > 10000 {
				actualFromBlock = currentBlock - 10000
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

		logs, err := g.client.FilterLogs(ctx, query)
		if err != nil {
			log.Printf("ERROR: Failed to scan past events: %v", err)
			return
		}

		log.Printf("Scanned %d past events (blocks %d-%d)", len(logs), actualFromBlock, actualToBlock)

		for _, vLog := range logs {
			if vLog.Address != g.contractAddress {
				continue
			}
			event := g.parseLog(vLog)
			if event != nil {
				log.Printf("Past event: %s itemId=%d tx=%s", event.Type, event.ItemId, event.TxHash)
				eventChan <- event
			}
		}
	}()

	return eventChan, nil
}

// parseLog はログをContractEventに変換
func (g *FrimaContractGateway) parseLog(vLog types.Log) *model.ContractEvent {
	if len(vLog.Topics) == 0 {
		log.Printf("Received log with no topics (tx: %s, address: %s)", vLog.TxHash.Hex(), vLog.Address.Hex())
		return nil
	}

	// コントラクトアドレスが一致するか確認
	if vLog.Address != g.contractAddress {
		log.Printf("Log address mismatch: expected %s, got %s (tx: %s)", g.contractAddress.Hex(), vLog.Address.Hex(), vLog.TxHash.Hex())
		return nil
	}

	eventSig := vLog.Topics[0].Hex()
	itemListedSig := g.contractABI.Events["ItemListed"].ID.Hex()
	itemPurchasedSig := g.contractABI.Events["ItemPurchased"].ID.Hex()
	itemUpdatedSig := g.contractABI.Events["ItemUpdated"].ID.Hex()
	itemCancelledSig := g.contractABI.Events["ItemCancelled"].ID.Hex()
	receiptConfirmedSig := g.contractABI.Events["ReceiptConfirmed"].ID.Hex()

	switch eventSig {
	case itemListedSig:
		return g.parseItemListed(vLog)
	case itemPurchasedSig:
		return g.parseItemPurchased(vLog)
	case itemUpdatedSig:
		return g.parseItemUpdated(vLog)
	case itemCancelledSig:
		return g.parseItemCancelled(vLog)
	case receiptConfirmedSig:
		return g.parseReceiptConfirmed(vLog)
	default:
		// 未知のイベントシグネチャをログに記録（デバッグ用）
		log.Printf("WARNING: Unknown event signature: %s (tx: %s, block: %d, address: %s). This might be from another contract or a different event.",
			eventSig, vLog.TxHash.Hex(), vLog.BlockNumber, vLog.Address.Hex())
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
	} else {
		log.Printf("WARNING: uid not found in ItemListed event")
	}
	if createdAt, ok := data["createdAt"].(*big.Int); ok {
		event.CreatedAt = createdAt.Uint64()
	}
	if category, ok := data["category"].(string); ok {
		event.Category = category
	}

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
