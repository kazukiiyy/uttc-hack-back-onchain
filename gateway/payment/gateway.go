package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"uttc-hack-back-onchain/model"
)

// デモ用: 固定の支払い金額 (0.001 ETH in Wei)
var DemoPaymentAmount = new(big.Int).Mul(big.NewInt(1), big.NewInt(1e15)) // 0.001 ETH

// ===============================================
// 1. インターフェース定義
// ===============================================

type BlockchainGateway interface {
	// GetProductPrice は商品の価格（円）を取得する
	GetProductPrice(productID string) (int, error)

	// GetRequiredAmount は支払いに必要なETH量を返す (デモ用: 0.001 ETH固定)
	GetRequiredAmount() *big.Int

	// GetPaymentAddress はアプリの集金用ウォレットアドレスを返す
	GetPaymentAddress() string

	// CheckPaymentStatus はトランザクションハッシュを受け取り、支払い完了を検証・確定する
	CheckPaymentStatus(ctx context.Context, txHash string, expectedAddr string, expectedWei *big.Int) (model.OrderStatus, error)
}

// ===============================================
// 2. 実装: EthGateway
// ===============================================

type EthGateway struct {
	client           *ethclient.Client
	appCollectWallet common.Address // アプリの集金用ウォレットアドレス
	backendBaseURL   string         // uttc-hackathon-backend のベースURL
}

// NewEthGateway は ethclient.Client を受け取る
func NewEthGateway(client *ethclient.Client, collectAddr string, backendBaseURL string) *EthGateway {
	return &EthGateway{
		client:           client,
		appCollectWallet: common.HexToAddress(collectAddr),
		backendBaseURL:   backendBaseURL,
	}
}

// ItemResponse はバックエンドからの商品レスポンス
type ItemResponse struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Price       int      `json:"price"`
	Explanation string   `json:"explanation"`
	ImageURLs   []string `json:"image_urls"`
	UID         string   `json:"uid"`
	IfPurchased bool     `json:"ifPurchased"`
	Category    string   `json:"category"`
	LikeCount   int      `json:"like_count"`
	CreatedAt   string   `json:"created_at"`
}

// GetProductPrice は商品IDからバックエンドAPIを呼び出し、価格（円）を取得する
func (g *EthGateway) GetProductPrice(productID string) (int, error) {
	url := fmt.Sprintf("%s/getItems/%s", g.backendBaseURL, productID)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching product %s: %v", productID, err)
		return 0, errors.New("failed to fetch product information")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Backend returned status %d for product %s", resp.StatusCode, productID)
		return 0, errors.New("product not found")
	}

	var item ItemResponse
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		log.Printf("Error decoding product response: %v", err)
		return 0, errors.New("failed to parse product information")
	}

	log.Printf("Product %s: %s - %d JPY", productID, item.Title, item.Price)
	return item.Price, nil
}

// GetRequiredAmount はデモ用の固定金額を返す (0.001 ETH)
func (g *EthGateway) GetRequiredAmount() *big.Int {
	return DemoPaymentAmount
}

// GetPaymentAddress (集金アドレスを文字列で返す)
func (g *EthGateway) GetPaymentAddress() string {
	return g.appCollectWallet.Hex()
}

// CheckPaymentStatus はETH送金トランザクションを検証する
func (g *EthGateway) CheckPaymentStatus(ctx context.Context, txHash string, expectedAddr string, expectedWei *big.Int) (model.OrderStatus, error) {
	// 1. TxHashを検証可能な型に変換
	txHashObj := common.HexToHash(txHash)
	if txHashObj.Big().Cmp(big.NewInt(0)) == 0 {
		return model.StatusError, errors.New("invalid transaction hash format")
	}

	expectedAddrObj := common.HexToAddress(expectedAddr)

	// 2. トランザクションが存在するか、Pendingでないかを確認
	tx, isPending, err := g.client.TransactionByHash(ctx, txHashObj)
	if err != nil {
		log.Printf("Error retrieving transaction %s: %v", txHash, err)
		return model.StatusError, errors.New("transaction not found or node error")
	}
	if isPending {
		return model.StatusPending, errors.New("transaction is still pending")
	}

	// 3. レシートを取得し、Txが成功したかを確認
	receipt, err := g.client.TransactionReceipt(ctx, txHashObj)
	if err != nil {
		log.Printf("Error retrieving receipt for transaction %s: %v", txHash, err)
		return model.StatusError, errors.New("failed to get transaction receipt")
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return model.StatusError, errors.New("transaction failed on chain (reverted)")
	}

	// 4. 送金額 (Value) の検証 - 期待額以上であればOK
	if tx.Value().Cmp(expectedWei) < 0 {
		log.Printf("Insufficient payment: got %s, expected %s", tx.Value().String(), expectedWei.String())
		return model.StatusError, errors.New("insufficient payment amount")
	}

	// 5. 送金先アドレス (To Address) の検証
	if tx.To() == nil {
		return model.StatusError, errors.New("transaction is not a transfer to a valid address")
	}
	if *tx.To() != expectedAddrObj {
		return model.StatusError, errors.New("transaction sent to wrong recipient address")
	}

	log.Printf("Payment verified: %s Wei to %s", tx.Value().String(), expectedAddr)
	return model.StatusPaid, nil
}
