package main

import (
	"context"
	"log"
	"net/http"
	"os"

	contractGateway "uttc-hack-back-onchain/gateway/contract"
	paymentGateway "uttc-hack-back-onchain/gateway/payment"
	contractHandler "uttc-hack-back-onchain/handler/contract"
	paymentHandler "uttc-hack-back-onchain/handler/payment"
	contractUsecase "uttc-hack-back-onchain/usecase/contract"
	paymentUsecase "uttc-hack-back-onchain/usecase/payment"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	// --- 1. 初期設定 ---
	nodeURL := os.Getenv("INFURA_SEPOLIA_URL")
	if nodeURL == "" {
		log.Fatal("INFURA_SEPOLIA_URL environment variable not set")
	}

	// WebSocket URL（イベント購読用）
	nodeWSURL := os.Getenv("INFURA_SEPOLIA_WS_URL")
	if nodeWSURL == "" {
		// HTTPからWSに変換を試みる
		nodeWSURL = nodeURL
	}

	appCollectAddr := os.Getenv("APP_COLLECT_WALLET_ADDRESS")
	if appCollectAddr == "" {
		log.Fatal("APP_COLLECT_WALLET_ADDRESS environment variable not set")
	}

	// スマートコントラクトアドレス
	marketplaceAddr := os.Getenv("MARKETPLACE_CONTRACT_ADDRESS")
	if marketplaceAddr == "" {
		log.Println("WARNING: MARKETPLACE_CONTRACT_ADDRESS not set. Contract features will be disabled.")
	}

	backendBaseURL := os.Getenv("BACKEND_BASE_URL")
	if backendBaseURL == "" {
		backendBaseURL = "https://hackathon-backend-982651832089.europe-west1.run.app"
	}

	// --- 2. ethclientの初期化 ---
	client, err := ethclient.Dial(nodeURL)
	if err != nil {
		log.Fatalf("Failed to connect to Sepolia network: %v", err)
	}
	log.Println("Successfully connected to Sepolia network (HTTP).")

	// --- 3. Payment機能の依存性注入 ---
	bcGateway := paymentGateway.NewEthGateway(client, appCollectAddr, backendBaseURL)
	log.Printf("Payment Address: %s", appCollectAddr)
	log.Printf("Backend URL: %s", backendBaseURL)
	log.Printf("Demo Payment Amount: 0.001 ETH (Sepolia)")

	paymentUC := paymentUsecase.NewPaymentUsecase(bcGateway)
	paymentHdlr := paymentHandler.NewPaymentHandler(paymentUC)

	// --- 4. Contract機能の依存性注入 ---
	var contractHdlr *contractHandler.ContractHandler

	if marketplaceAddr != "" {
		// WebSocket接続でイベント購読
		wsClient, err := ethclient.Dial(nodeWSURL)
		if err != nil {
			log.Printf("WARNING: Failed to connect WebSocket for events: %v", err)
			// HTTP clientでコントラクト機能は使用可能
			wsClient = client
		} else {
			log.Println("Successfully connected to Sepolia network (WebSocket for events).")
		}

		ctGateway, err := contractGateway.NewFrimaContractGateway(wsClient, marketplaceAddr)
		if err != nil {
			log.Printf("WARNING: Failed to initialize contract gateway: %v", err)
		} else {
			log.Printf("Marketplace Contract: %s", marketplaceAddr)

			contractUC := contractUsecase.NewContractUsecase(ctGateway, backendBaseURL)
			contractHdlr = contractHandler.NewContractHandler(contractUC)

			// イベントリスナーを開始
			ctx := context.Background()
			if err := contractUC.StartEventListener(ctx); err != nil {
				log.Printf("WARNING: Failed to start event listener: %v", err)
			} else {
				log.Println("Contract event listener started successfully.")
			}
		}
	}

	// --- 5. ルーティングの設定 ---
	router := mux.NewRouter()

	// ヘルスチェック用エンドポイント
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Payment API
	router.HandleFunc("/api/v1/payment/order", paymentHdlr.HandleCreatePaymentOrder).Methods("POST")
	router.HandleFunc("/api/v1/payment/confirm", paymentHdlr.HandleConfirmPayment).Methods("POST")

	// Contract API
	if contractHdlr != nil {
		router.HandleFunc("/api/v1/contract/info", contractHdlr.HandleContractInfo).Methods("GET")
		router.HandleFunc("/api/v1/contract/item/{itemId}", contractHdlr.HandleGetItem).Methods("GET")
		router.HandleFunc("/api/v1/contract/verify-tx", contractHdlr.HandleVerifyTransaction).Methods("POST")
	}

	// --- 6. CORSミドルウェアの設定 ---
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})
	corsHandler := c.Handler(router)

	// --- 7. サーバー起動 ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Onchain Service (Sepolia) starting on :%s", port)
	log.Println("Available endpoints:")
	log.Println("  - GET  /health")
	log.Println("  - POST /api/v1/payment/order")
	log.Println("  - POST /api/v1/payment/confirm")
	if contractHdlr != nil {
		log.Println("  - GET  /api/v1/contract/info")
		log.Println("  - GET  /api/v1/contract/item/{itemId}")
		log.Println("  - POST /api/v1/contract/verify-tx")
	}

	if err := http.ListenAndServe(":"+port, corsHandler); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
