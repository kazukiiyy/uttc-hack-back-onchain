package model

import (
	"time"
)

// OrderStatus は注文の状態を表す列挙型
type OrderStatus string

const (
	StatusPending OrderStatus = "PENDING"       // 支払い待ち
	StatusPaid    OrderStatus = "PAID"          // 支払い済み
	StatusShipped OrderStatus = "SHIPPED"       // 発送済み
	StatusError   OrderStatus = "PAYMENT_ERROR" // 支払いエラー
)

// PaymentOrder は決済に必要な最小限の注文情報
type PaymentOrder struct {
	OrderID     string      `json:"order_id"`      // 注文ID (ユニーク)
	ProductID   string      `json:"product_id"`    // 商品ID
	ProductName string      `json:"product_name"`  // 商品名
	PriceYen    int         `json:"price_yen"`     // 商品価格（円）
	AmountETH   string      `json:"amount_eth"`    // 支払い金額 (ETH表示用)
	AmountWei   string      `json:"amount_wei"`    // 支払い金額 (Wei)
	PaymentAddr string      `json:"payment_addr"`  // 支払い先ウォレットアドレス
	BuyerWallet string      `json:"buyer_wallet"`  // 購入者のウォレットアドレス
	Status      OrderStatus `json:"status"`        // 注文ステータス
	TxHash      string      `json:"tx_hash"`       // トランザクションハッシュ
	CreatedAt   time.Time   `json:"created_at"`
}
