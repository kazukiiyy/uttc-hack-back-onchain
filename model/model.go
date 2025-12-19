package model

import (
	"math/big"
	"time"
)

// OrderStatus は注文の状態を表す列挙型
type OrderStatus string

const (
	StatusPending OrderStatus = "PENDING"       // 支払い待ち
	StatusPaid    OrderStatus = "PAID"          // 支払い済み
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

// ===============================================
// スマートコントラクト関連のモデル
// ===============================================

// EventType はコントラクトイベントの種類
type EventType string

const (
	EventItemListed       EventType = "ItemListed"
	EventItemPurchased    EventType = "ItemPurchased"
	EventItemUpdated      EventType = "ItemUpdated"
	EventItemCancelled    EventType = "ItemCancelled"
	EventReceiptConfirmed EventType = "ReceiptConfirmed"
)

// ContractEvent はコントラクトイベントを表す
type ContractEvent struct {
	Type        EventType `json:"type"`
	TxHash      string    `json:"tx_hash"`
	BlockNo     uint64    `json:"block_number"`
	ItemId      uint64    `json:"item_id"`
	TokenId     uint64    `json:"token_id,omitempty"`
	Title       string    `json:"title,omitempty"`
	Price       *big.Int  `json:"price,omitempty"`
	Explanation string    `json:"explanation,omitempty"`
	ImageUrl    string    `json:"image_url,omitempty"`
	Uid         string    `json:"uid,omitempty"`
	Category    string    `json:"category,omitempty"`
	Seller      string    `json:"seller,omitempty"`
	Buyer       string    `json:"buyer,omitempty"`
	BuyerUid    string    `json:"buyer_uid,omitempty"`
	CreatedAt   uint64    `json:"created_at,omitempty"`
	UpdatedAt   uint64    `json:"updated_at,omitempty"`
}

// ContractItem はコントラクトの商品情報
type ContractItem struct {
	ItemId      uint64   `json:"item_id"`
	TokenId     uint64   `json:"token_id"`
	Title       string   `json:"title"`
	Price       *big.Int `json:"price"`
	Explanation string   `json:"explanation"`
	ImageUrl    string   `json:"image_url"`
	Uid         string   `json:"uid"`
	CreatedAt   uint64   `json:"created_at"`
	UpdatedAt   uint64   `json:"updated_at"`
	IsPurchased bool     `json:"is_purchased"`
	Category    string   `json:"category"`
	Seller      string   `json:"seller"`
	Buyer       string   `json:"buyer"`
	BuyerUid    string   `json:"buyer_uid"`
	Status      uint8    `json:"status"` // 0: Listed, 1: Purchased, 2: Completed, 3: Cancelled
}

// TxVerification はトランザクション検証結果
type TxVerification struct {
	TxHash         string `json:"tx_hash"`
	Status         string `json:"status"` // "pending", "success", "failed"
	BlockNumber    uint64 `json:"block_number,omitempty"`
	GasUsed        uint64 `json:"gas_used,omitempty"`
	Success        bool   `json:"success"`
	IsContractCall bool   `json:"is_contract_call"`
}
