package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"uttc-hack-back-onchain/usecase/contract"

	"github.com/gorilla/mux"
)

type ContractHandler struct {
	contractUC usecase.ContractUsecase
}

func NewContractHandler(uc usecase.ContractUsecase) *ContractHandler {
	return &ContractHandler{contractUC: uc}
}

// HandleGetItem はコントラクトから商品情報を取得
func (h *ContractHandler) HandleGetItem(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemIdStr := vars["itemId"]

	itemId, err := strconv.ParseUint(itemIdStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	item, err := h.contractUC.GetItem(r.Context(), itemId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Priceをstring形式に変換してレスポンス
	response := map[string]interface{}{
		"item_id":      item.ItemId,
		"token_id":     item.TokenId,
		"title":        item.Title,
		"price_wei":    item.Price.String(),
		"explanation":  item.Explanation,
		"image_url":    item.ImageUrl,
		"uid":          item.Uid,
		"created_at":   item.CreatedAt,
		"updated_at":   item.UpdatedAt,
		"is_purchased": item.IsPurchased,
		"category":     item.Category,
		"seller":       item.Seller,
		"buyer":        item.Buyer,
		"status":       item.Status,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// VerifyTxRequest はトランザクション検証リクエスト
type VerifyTxRequest struct {
	TxHash string `json:"tx_hash"`
}

// HandleVerifyTransaction はトランザクションを検証
func (h *ContractHandler) HandleVerifyTransaction(w http.ResponseWriter, r *http.Request) {
	var req VerifyTxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TxHash == "" {
		http.Error(w, "tx_hash is required", http.StatusBadRequest)
		return
	}

	verification, err := h.contractUC.VerifyTransaction(r.Context(), req.TxHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(verification)
}

// HandleContractInfo はコントラクト情報を返す
func (h *ContractHandler) HandleContractInfo(w http.ResponseWriter, r *http.Request) {
	// ContractUsecaseからコントラクトアドレスを取得する場合は別途メソッド追加が必要
	// ここでは簡易的にハンドラーで保持している情報を返す
	info := map[string]string{
		"message": "Contract API is running",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
