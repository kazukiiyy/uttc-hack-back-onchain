package usecase

import (
	"bytes"
	"context"
	"encoding/json"
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
	eventChan, err := uc.gateway.SubscribeEvents(ctx)
	if err != nil {
		return err
	}

	go func() {
		for event := range eventChan {
			uc.handleEvent(event)
		}
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
func (uc *contractUsecase) notifyBackend(endpoint string, payload interface{}) error {
	url := uc.backendBaseURL + endpoint

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("Backend returned status %d for %s", resp.StatusCode, endpoint)
	}

	return nil
}

// GetItem はコントラクトから商品情報を取得
func (uc *contractUsecase) GetItem(ctx context.Context, itemId uint64) (*model.ContractItem, error) {
	return uc.gateway.GetItem(ctx, itemId)
}

// VerifyTransaction はトランザクションを検証
func (uc *contractUsecase) VerifyTransaction(ctx context.Context, txHash string) (*model.TxVerification, error) {
	return uc.gateway.VerifyTransaction(ctx, txHash)
}
