package main

import (
	"log"
	"net/http"
	"os"

	gateway "uttc-hack-back-onchain/gateway/payment"
	handler "uttc-hack-back-onchain/handler/payment"
	usecase "uttc-hack-back-onchain/usecase/payment"

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

	appCollectAddr := os.Getenv("APP_COLLECT_WALLET_ADDRESS")
	if appCollectAddr == "" {
		log.Fatal("APP_COLLECT_WALLET_ADDRESS environment variable not set")
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
	log.Println("Successfully connected to Sepolia network.")

	// --- 3. 依存性の注入 (DI) ---
	bcGateway := gateway.NewEthGateway(client, appCollectAddr, backendBaseURL)
	log.Printf("Payment Address: %s", appCollectAddr)
	log.Printf("Backend URL: %s", backendBaseURL)
	log.Printf("Demo Payment Amount: 0.001 ETH (Sepolia)")

	paymentUC := usecase.NewPaymentUsecase(bcGateway)
	paymentHandler := handler.NewPaymentHandler(paymentUC)

	// --- 4. ルーティングの設定 ---
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/payment/order", paymentHandler.HandleCreatePaymentOrder).Methods("POST")
	router.HandleFunc("/api/v1/payment/confirm", paymentHandler.HandleConfirmPayment).Methods("POST")

	// --- 5. CORSミドルウェアの設定 ---
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})
	corsHandler := c.Handler(router)

	// --- 6. サーバー起動 ---
	log.Println("Crypto Payment Service (Sepolia Demo) starting on :8080")
	if err := http.ListenAndServe(":8080", corsHandler); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
