package usecase

import (
	"context"
	"errors"
	"time"

	"uttc-hack-back-onchain/gateway/payment"
	"uttc-hack-back-onchain/model"
)

// PaymentUsecase は決済処理のビジネスロジックを定義
type PaymentUsecase interface {
	// CreatePaymentOrder は支払い情報を初期化し、フロントエンドに返すべき情報を生成する
	CreatePaymentOrder(ctx context.Context, productID string, buyerWallet string) (*model.PaymentOrder, error)

	// ConfirmPayment はトランザクションハッシュを受け取り、支払い完了を検証・確定する
	ConfirmPayment(ctx context.Context, orderID string, productID string, txHash string) (*model.PaymentOrder, error)
}

type paymentUsecase struct {
	bcGateway gateway.BlockchainGateway
}

func NewPaymentUsecase(bc gateway.BlockchainGateway) *paymentUsecase {
	return &paymentUsecase{
		bcGateway: bc,
	}
}

func (uc *paymentUsecase) CreatePaymentOrder(ctx context.Context, productID string, buyerWallet string) (*model.PaymentOrder, error) {
	// 1. バックエンドから商品価格（円）を取得
	priceYen, err := uc.bcGateway.GetProductPrice(productID)
	if err != nil {
		return nil, err
	}

	// 2. 支払い金額を取得 (デモ用: 0.001 ETH固定)
	amountWei := uc.bcGateway.GetRequiredAmount()
	paymentAddr := uc.bcGateway.GetPaymentAddress()

	// 3. 注文モデルを作成
	newOrder := &model.PaymentOrder{
		OrderID:     "ORDER-" + productID + "-" + time.Now().Format("20060102150405"),
		ProductID:   productID,
		PriceYen:    priceYen,
		AmountETH:   "0.001",
		AmountWei:   amountWei.String(),
		PaymentAddr: paymentAddr,
		BuyerWallet: buyerWallet,
		Status:      model.StatusPending,
		CreatedAt:   time.Now(),
	}

	return newOrder, nil
}

func (uc *paymentUsecase) ConfirmPayment(ctx context.Context, orderID string, productID string, txHash string) (*model.PaymentOrder, error) {
	// 1. 商品価格を再取得（商品が存在するか確認）
	priceYen, err := uc.bcGateway.GetProductPrice(productID)
	if err != nil {
		return nil, errors.New("failed to get product: " + err.Error())
	}

	// 2. 支払い金額を取得 (デモ用: 0.001 ETH固定)
	expectedAmount := uc.bcGateway.GetRequiredAmount()
	paymentAddr := uc.bcGateway.GetPaymentAddress()

	// 3. 注文情報を構築
	order := &model.PaymentOrder{
		OrderID:     orderID,
		ProductID:   productID,
		PriceYen:    priceYen,
		AmountETH:   "0.001",
		AmountWei:   expectedAmount.String(),
		PaymentAddr: paymentAddr,
		TxHash:      txHash,
	}

	// 4. ブロックチェーン上でトランザクションを検証
	newStatus, err := uc.bcGateway.CheckPaymentStatus(ctx, txHash, paymentAddr, expectedAmount)
	if err != nil {
		order.Status = model.StatusError
		return nil, errors.New("payment verification failed: " + err.Error())
	}

	order.Status = newStatus
	return order, nil
}
