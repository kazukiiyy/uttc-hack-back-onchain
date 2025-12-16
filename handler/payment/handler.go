package handler

import (
	"encoding/json"
	"net/http"

	"uttc-hack-back-onchain/usecase/payment"
)

type PaymentHandler struct {
	paymentUC usecase.PaymentUsecase
}

func NewPaymentHandler(uc usecase.PaymentUsecase) *PaymentHandler {
	return &PaymentHandler{paymentUC: uc}
}

// CreateOrderRequest は決済開始APIの入力
type CreateOrderRequest struct {
	ProductID   string `json:"product_id"`
	BuyerWallet string `json:"buyer_wallet"`
}

// HandleCreatePaymentOrder は注文を作成し、必要な支払い情報を返す
func (h *PaymentHandler) HandleCreatePaymentOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Usecaseにビジネスロジックを委譲
	order, err := h.paymentUC.CreatePaymentOrder(r.Context(), req.ProductID, req.BuyerWallet)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// ConfirmPaymentRequest は支払い確定APIの入力
type ConfirmPaymentRequest struct {
	OrderID   string `json:"order_id"`
	ProductID string `json:"product_id"`
	TxHash    string `json:"tx_hash"`
}

// HandleConfirmPayment はJPYC支払いトランザクションを検証する
func (h *PaymentHandler) HandleConfirmPayment(w http.ResponseWriter, r *http.Request) {
	var req ConfirmPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Usecaseにビジネスロジックを委譲
	order, err := h.paymentUC.ConfirmPayment(r.Context(), req.OrderID, req.ProductID, req.TxHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}
