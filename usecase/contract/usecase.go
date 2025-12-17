package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"uttc-hack-back-onchain/gateway/contract"
	"uttc-hack-back-onchain/model"
)

// ContractUsecase はスマートコントラクト関連のビジネスロジック
type ContractUsecase interface {
	// StartEventListener はイベントリスナーを開始
	StartEventListener(ctx context.Context) error

	// GetItem はコントラクトから商品情報を取得
	GetItem(ctx context.Context, itemId uint64) (*model.ContractItem, error)

	// VerifyTransaction はトランザクションを検証
	VerifyTransaction(ctx context.Context, txHash string) (*model.TxVerification, error)
}

type contractUsecase struct {
	gateway        contract.ContractGateway
	backendBaseURL string
}

func NewContractUsecase(gw contract.ContractGateway, backendBaseURL string) *contractUsecase {
	return &contractUsecase{
		gateway:        gw,
		backendBaseURL: backendBaseURL,
	}
}

// StartEventListener はイベントリスナーを開始し、イベントをメインバックエンドに通知
func (uc *contractUsecase) StartEventListener(ctx context.Context) error {
	// まず過去のイベントをスキャン（最後の1000ブロック）
	// これにより、リスナー起動後に発火したイベントもキャッチできる
	go func() {
		// 過去のイベントをスキャン（fromBlock=0は最後の1000ブロックからスキャンすることを意味）
		// 注意: 実際の実装では、コントラクトのデプロイブロックからスキャンする方が良い
		log.Printf("Starting to scan past events (last 1000 blocks)...")
		
		pastEvents, err := uc.gateway.ScanPastEvents(ctx, 0, nil)
		if err != nil {
			log.Printf("Failed to scan past events: %v", err)
		} else {
			eventCount := 0
			for event := range pastEvents {
				eventCount++
				log.Printf("Processing past event: %s for item %d (tx: %s)", event.Type, event.ItemId, event.TxHash)
				uc.handleEvent(event)
			}
			log.Printf("Processed %d past events", eventCount)
		}
	}()

	// リアルタイムイベントを購読
	eventChan, err := uc.gateway.SubscribeEvents(ctx)
	if err != nil {
		log.Printf("Failed to subscribe to events: %v", err)
		return err
	}

	go func() {
		log.Println("Starting real-time event subscription...")
		for event := range eventChan {
			log.Printf("Processing real-time event: %s for item %d (tx: %s)", event.Type, event.ItemId, event.TxHash)
			uc.handleEvent(event)
		}
		log.Println("Real-time event subscription ended")
	}()

	log.Println("Contract event listener started")
	return nil
}

// handleEvent はイベントを処理してメインバックエンドに通知
func (uc *contractUsecase) handleEvent(event *model.ContractEvent) {
	log.Printf("Received event: %s for item %d (tx: %s)", event.Type, event.ItemId, event.TxHash)

	var endpoint string
	var payload interface{}

	switch event.Type {
	case model.EventItemListed:
		endpoint = "/api/v1/blockchain/item-listed"
		log.Printf("Preparing ItemListed payload: chain_item_id=%d, uid=%s, title=%s", event.ItemId, event.Uid, event.Title)
		if event.Uid == "" {
			log.Printf("WARNING: uid is empty in ItemListed event for item %d", event.ItemId)
		}
		payload = map[string]interface{}{
			"chain_item_id": event.ItemId,
			"token_id":      event.TokenId,
			"title":         event.Title,
			"price_wei":     event.Price.String(),
			"explanation":   event.Explanation,
			"image_url":     event.ImageUrl,
			"uid":           event.Uid,
			"category":      event.Category,
			"seller":        event.Seller,
			"created_at":    event.CreatedAt,
			"tx_hash":       event.TxHash,
		}

	case model.EventItemPurchased:
		endpoint = "/api/v1/blockchain/item-purchased"
		payload = map[string]interface{}{
			"chain_item_id": event.ItemId,
			"buyer":         event.Buyer,
			"price_wei":     event.Price.String(),
			"token_id":      event.TokenId,
			"tx_hash":       event.TxHash,
		}

	case model.EventItemUpdated:
		endpoint = "/api/v1/blockchain/item-updated"
		payload = map[string]interface{}{
			"chain_item_id": event.ItemId,
			"title":         event.Title,
			"price_wei":     event.Price.String(),
			"explanation":   event.Explanation,
			"image_url":     event.ImageUrl,
			"category":      event.Category,
			"updated_at":    event.UpdatedAt,
			"tx_hash":       event.TxHash,
		}

	case model.EventItemCancelled:
		endpoint = "/api/v1/blockchain/item-cancelled"
		payload = map[string]interface{}{
			"chain_item_id": event.ItemId,
			"seller":        event.Seller,
			"tx_hash":       event.TxHash,
		}

	case model.EventReceiptConfirmed:
		endpoint = "/api/v1/blockchain/receipt-confirmed"
		payload = map[string]interface{}{
			"chain_item_id": event.ItemId,
			"buyer":         event.Buyer,
			"seller":        event.Seller,
			"price_wei":     event.Price.String(),
			"tx_hash":       event.TxHash,
		}

	default:
		log.Printf("Unknown event type: %s", event.Type)
		return
	}

	// メインバックエンドに通知
	if err := uc.notifyBackend(endpoint, payload); err != nil {
		log.Printf("Failed to notify backend for event %s: %v", event.Type, err)
	} else {
		log.Printf("Successfully notified backend for event %s (item %d)", event.Type, event.ItemId)
	}
}

// notifyBackend はメインバックエンドにイベントを通知
// リトライロジック付き（最大3回）
func (uc *contractUsecase) notifyBackend(endpoint string, payload interface{}) error {
	url := uc.backendBaseURL + endpoint

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			// リトライ前に待機（指数バックオフ）
			waitTime := time.Duration(i) * time.Second
			log.Printf("Retrying backend notification (attempt %d/%d) after %v...", i+1, maxRetries, waitTime)
			time.Sleep(waitTime)
		}

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = fmt.Errorf("failed to send request: %w", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil // 成功
		}

		// レスポンスボディを読み取ってエラー詳細を取得
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		bodyStr := string(bodyBytes[:n])

		lastErr = fmt.Errorf("backend returned status %d: %s", resp.StatusCode, bodyStr)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// 4xxエラーはリトライしない
			return lastErr
		}
		// 5xxエラーやネットワークエラーはリトライ
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// GetItem はコントラクトから商品情報を取得
func (uc *contractUsecase) GetItem(ctx context.Context, itemId uint64) (*model.ContractItem, error) {
	return uc.gateway.GetItem(ctx, itemId)
}

// VerifyTransaction はトランザクションを検証
func (uc *contractUsecase) VerifyTransaction(ctx context.Context, txHash string) (*model.TxVerification, error) {
	return uc.gateway.VerifyTransaction(ctx, txHash)
}
