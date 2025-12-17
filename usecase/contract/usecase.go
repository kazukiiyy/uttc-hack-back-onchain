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
	log.Println("=== Starting Event Listener ===")
	log.Printf("Backend Base URL: %s", uc.backendBaseURL)
	log.Printf("Contract Address: %s", uc.gateway.GetContractAddress())

	// まず過去のイベントをスキャン（最後の1000ブロック）
	// これにより、リスナー起動後に発火したイベントもキャッチできる
	go func() {
		// 過去のイベントをスキャン（fromBlock=0は最後の1000ブロックからスキャンすることを意味）
		// 注意: 実際の実装では、コントラクトのデプロイブロックからスキャンする方が良い
		log.Printf("=== Starting to scan past events (last 1000 blocks) ===")
		
		pastEvents, err := uc.gateway.ScanPastEvents(ctx, 0, nil)
		if err != nil {
			log.Printf("ERROR: Failed to scan past events: %v", err)
		} else {
			eventCount := 0
			for event := range pastEvents {
				eventCount++
				log.Printf("=== Processing past event #%d ===", eventCount)
				log.Printf("  Event Type: %s", event.Type)
				log.Printf("  Item ID: %d", event.ItemId)
				log.Printf("  TX Hash: %s", event.TxHash)
				uc.handleEvent(event)
			}
			log.Printf("=== Finished scanning past events: Processed %d events ===", eventCount)
		}
	}()

	// リアルタイムイベントを購読
	log.Printf("=== Starting real-time event subscription ===")
	eventChan, err := uc.gateway.SubscribeEvents(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to subscribe to events: %v", err)
		return err
	}

	go func() {
		log.Println("=== Real-time event subscription started ===")
		eventCount := 0
		for event := range eventChan {
			eventCount++
			log.Printf("=== Processing real-time event #%d ===", eventCount)
			log.Printf("  Event Type: %s", event.Type)
			log.Printf("  Item ID: %d", event.ItemId)
			log.Printf("  TX Hash: %s", event.TxHash)
			uc.handleEvent(event)
		}
		log.Println("=== Real-time event subscription ended ===")
	}()

	log.Println("=== Contract event listener started successfully ===")
	return nil
}

// handleEvent はイベントを処理してメインバックエンドに通知
func (uc *contractUsecase) handleEvent(event *model.ContractEvent) {
	log.Printf("=== handleEvent: Received event: %s for item %d (tx: %s, block: %d) ===", event.Type, event.ItemId, event.TxHash, event.BlockNo)

	var endpoint string
	var payload interface{}

	switch event.Type {
	case model.EventItemListed:
		endpoint = "/api/v1/blockchain/item-listed"
		log.Printf("Processing ItemListed event: chain_item_id=%d, uid=%s, title=%s, seller=%s", event.ItemId, event.Uid, event.Title, event.Seller)
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
		log.Printf("ItemListed payload prepared: %+v", payload)

	case model.EventItemPurchased:
		endpoint = "/api/v1/blockchain/item-purchased"
		log.Printf("Processing ItemPurchased event: chain_item_id=%d, buyer=%s", event.ItemId, event.Buyer)
		payload = map[string]interface{}{
			"chain_item_id": event.ItemId,
			"buyer":         event.Buyer,
			"price_wei":     event.Price.String(),
			"token_id":      event.TokenId,
			"tx_hash":       event.TxHash,
		}
		log.Printf("ItemPurchased payload prepared: %+v", payload)

	case model.EventItemUpdated:
		endpoint = "/api/v1/blockchain/item-updated"
		log.Printf("Processing ItemUpdated event: chain_item_id=%d", event.ItemId)
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
		log.Printf("ItemUpdated payload prepared: %+v", payload)

	case model.EventItemCancelled:
		endpoint = "/api/v1/blockchain/item-cancelled"
		log.Printf("Processing ItemCancelled event: chain_item_id=%d, seller=%s", event.ItemId, event.Seller)
		payload = map[string]interface{}{
			"chain_item_id": event.ItemId,
			"seller":        event.Seller,
			"tx_hash":       event.TxHash,
		}
		log.Printf("ItemCancelled payload prepared: %+v", payload)

	case model.EventReceiptConfirmed:
		endpoint = "/api/v1/blockchain/receipt-confirmed"
		log.Printf("Processing ReceiptConfirmed event: chain_item_id=%d, buyer=%s, seller=%s", event.ItemId, event.Buyer, event.Seller)
		payload = map[string]interface{}{
			"chain_item_id": event.ItemId,
			"buyer":         event.Buyer,
			"seller":        event.Seller,
			"price_wei":     event.Price.String(),
			"tx_hash":       event.TxHash,
		}
		log.Printf("ReceiptConfirmed payload prepared: %+v", payload)

	default:
		log.Printf("ERROR: Unknown event type: %s", event.Type)
		return
	}

	// メインバックエンドに通知
	log.Printf("Calling notifyBackend for event %s (endpoint: %s)", event.Type, endpoint)
	if err := uc.notifyBackend(endpoint, payload); err != nil {
		log.Printf("ERROR: Failed to notify backend for event %s (item %d): %v", event.Type, event.ItemId, err)
	} else {
		log.Printf("SUCCESS: Successfully notified backend for event %s (item %d)", event.Type, event.ItemId)
	}
	log.Printf("=== handleEvent completed for event %s (item %d) ===", event.Type, event.ItemId)
}

// notifyBackend はメインバックエンドにイベントを通知
// リトライロジック付き（最大3回）
func (uc *contractUsecase) notifyBackend(endpoint string, payload interface{}) error {
	url := uc.backendBaseURL + endpoint

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: Failed to marshal payload: %v", err)
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	log.Printf("POST Request to backend: %s", url)
	log.Printf("POST Payload: %s", string(jsonData))

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

		log.Printf("Sending POST request (attempt %d/%d) to %s", i+1, maxRetries, url)
		resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = fmt.Errorf("failed to send request: %w", err)
			log.Printf("ERROR: POST request failed: %v", err)
			continue
		}
		defer resp.Body.Close()

		log.Printf("POST Response: Status=%d, StatusCode=%d", resp.StatusCode, resp.StatusCode)

		// レスポンスボディを読み取ってエラー詳細を取得
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		bodyStr := string(bodyBytes[:n])
		if n > 0 {
			log.Printf("POST Response Body: %s", bodyStr)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("POST request succeeded: Status=%d", resp.StatusCode)
			return nil // 成功
		}

		lastErr = fmt.Errorf("backend returned status %d: %s", resp.StatusCode, bodyStr)
		log.Printf("ERROR: POST request failed with status %d: %s", resp.StatusCode, bodyStr)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// 4xxエラーはリトライしない
			return lastErr
		}
		// 5xxエラーやネットワークエラーはリトライ
	}

	log.Printf("ERROR: POST request failed after %d retries: %v", maxRetries, lastErr)
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
