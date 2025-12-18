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
	log.Printf("Starting event listener (backend: %s, contract: %s)", uc.backendBaseURL, uc.gateway.GetContractAddress())

	// 過去のイベントをスキャン（完了後にリアルタイムリスニングを開始）
	go func() {
		pastEvents, err := uc.gateway.ScanPastEvents(ctx, 0, nil)
		if err != nil {
			log.Printf("ERROR: Failed to scan past events: %v", err)
			// エラーが発生してもリアルタイムリスニングは開始する
			go uc.startRealtimeListener(ctx)
			return
		}
		
		// 過去のイベントを処理
		lastProcessedBlock := uint64(0)
		for event := range pastEvents {
			uc.handleEvent(event)
			if event.BlockNo > lastProcessedBlock {
				lastProcessedBlock = event.BlockNo
			}
		}
		
		log.Printf("Past events scan completed. Starting realtime listener from block %d", lastProcessedBlock)
		// 過去のイベントスキャンが完了したら、リアルタイムリスニングを開始
		go uc.startRealtimeListener(ctx)
	}()

	log.Println("Event listener started (past events scan in progress)")
	return nil
}

// startRealtimeListener はリアルタイムイベントリスニングを開始（自動再起動）
func (uc *contractUsecase) startRealtimeListener(ctx context.Context) {
	retryDelay := 5 * time.Second
	maxRetryDelay := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
			eventChan, err := uc.gateway.SubscribeEvents(ctx)
			if err != nil {
				log.Printf("ERROR: Failed to subscribe: %v, retrying in %v", err, retryDelay)
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryDelay):
					retryDelay = min(retryDelay*2, maxRetryDelay)
				}
				continue
			}

			retryDelay = 5 * time.Second
			log.Println("Event listener connected (realtime)")

			for event := range eventChan {
				uc.handleEvent(event)
			}

			log.Printf("Event channel closed, reconnecting in %v...", retryDelay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
				retryDelay = min(retryDelay*2, maxRetryDelay)
			}
		}
	}
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// handleEvent はイベントを処理してメインバックエンドに通知
func (uc *contractUsecase) handleEvent(event *model.ContractEvent) {
	var endpoint string
	var payload interface{}

	switch event.Type {
	case model.EventItemListed:
		endpoint = "/api/v1/blockchain/item-listed"
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
		log.Printf("ERROR: Unknown event type: %s", event.Type)
		return
	}

	if err := uc.notifyBackend(endpoint, payload); err != nil {
		log.Printf("ERROR: Failed to notify backend for event %s (item %d): %v", event.Type, event.ItemId, err)
	}
}

// notifyBackend はメインバックエンドにイベントを通知
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
			waitTime := time.Duration(i) * time.Second
			time.Sleep(waitTime)
		}

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = fmt.Errorf("failed to send request: %w", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		bodyStr := string(bodyBytes[:n])

		lastErr = fmt.Errorf("backend returned status %d: %s", resp.StatusCode, bodyStr)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return lastErr
		}
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
