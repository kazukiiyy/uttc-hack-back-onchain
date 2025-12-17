package contract

// FrimaMarketplaceABI はFrimaMarketplaceコントラクトのABI
const FrimaMarketplaceABI = `[
  {
    "inputs": [{"internalType": "address", "name": "_nftContract", "type": "address"}],
    "stateMutability": "nonpayable",
    "type": "constructor"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "internalType": "uint256", "name": "itemId", "type": "uint256"},
      {"indexed": true, "internalType": "address", "name": "seller", "type": "address"},
      {"indexed": false, "internalType": "uint256", "name": "timestamp", "type": "uint256"}
    ],
    "name": "ItemCancelled",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "internalType": "uint256", "name": "itemId", "type": "uint256"},
      {"indexed": true, "internalType": "uint256", "name": "tokenId", "type": "uint256"},
      {"indexed": true, "internalType": "address", "name": "seller", "type": "address"},
      {"indexed": false, "internalType": "string", "name": "title", "type": "string"},
      {"indexed": false, "internalType": "uint256", "name": "price", "type": "uint256"},
      {"indexed": false, "internalType": "string", "name": "explanation", "type": "string"},
      {"indexed": false, "internalType": "string", "name": "imageUrl", "type": "string"},
      {"indexed": false, "internalType": "string", "name": "uid", "type": "string"},
      {"indexed": false, "internalType": "uint256", "name": "createdAt", "type": "uint256"},
      {"indexed": false, "internalType": "string", "name": "category", "type": "string"}
    ],
    "name": "ItemListed",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "internalType": "uint256", "name": "itemId", "type": "uint256"},
      {"indexed": true, "internalType": "address", "name": "buyer", "type": "address"},
      {"indexed": false, "internalType": "uint256", "name": "price", "type": "uint256"},
      {"indexed": false, "internalType": "uint256", "name": "timestamp", "type": "uint256"},
      {"indexed": false, "internalType": "uint256", "name": "tokenId", "type": "uint256"}
    ],
    "name": "ItemPurchased",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "internalType": "uint256", "name": "itemId", "type": "uint256"},
      {"indexed": false, "internalType": "string", "name": "title", "type": "string"},
      {"indexed": false, "internalType": "uint256", "name": "price", "type": "uint256"},
      {"indexed": false, "internalType": "string", "name": "explanation", "type": "string"},
      {"indexed": false, "internalType": "string", "name": "imageUrl", "type": "string"},
      {"indexed": false, "internalType": "string", "name": "category", "type": "string"},
      {"indexed": false, "internalType": "uint256", "name": "updatedAt", "type": "uint256"}
    ],
    "name": "ItemUpdated",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "internalType": "uint256", "name": "itemId", "type": "uint256"},
      {"indexed": true, "internalType": "address", "name": "buyer", "type": "address"},
      {"indexed": true, "internalType": "address", "name": "seller", "type": "address"},
      {"indexed": false, "internalType": "uint256", "name": "price", "type": "uint256"},
      {"indexed": false, "internalType": "uint256", "name": "timestamp", "type": "uint256"}
    ],
    "name": "ReceiptConfirmed",
    "type": "event"
  },
  {
    "inputs": [{"internalType": "uint256", "name": "_itemId", "type": "uint256"}],
    "name": "buyItem",
    "outputs": [],
    "stateMutability": "payable",
    "type": "function"
  },
  {
    "inputs": [{"internalType": "uint256", "name": "_itemId", "type": "uint256"}],
    "name": "cancelListing",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [{"internalType": "uint256", "name": "_itemId", "type": "uint256"}],
    "name": "confirmReceipt",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [{"internalType": "uint256", "name": "_itemId", "type": "uint256"}],
    "name": "getItem",
    "outputs": [
      {
        "components": [
          {"internalType": "uint256", "name": "itemId", "type": "uint256"},
          {"internalType": "uint256", "name": "tokenId", "type": "uint256"},
          {"internalType": "string", "name": "title", "type": "string"},
          {"internalType": "uint256", "name": "price", "type": "uint256"},
          {"internalType": "string", "name": "explanation", "type": "string"},
          {"internalType": "string", "name": "imageUrl", "type": "string"},
          {"internalType": "string", "name": "uid", "type": "string"},
          {"internalType": "uint256", "name": "createdAt", "type": "uint256"},
          {"internalType": "uint256", "name": "updatedAt", "type": "uint256"},
          {"internalType": "bool", "name": "isPurchased", "type": "bool"},
          {"internalType": "string", "name": "category", "type": "string"},
          {"internalType": "address", "name": "seller", "type": "address"},
          {"internalType": "address", "name": "buyer", "type": "address"},
          {"internalType": "uint8", "name": "status", "type": "uint8"}
        ],
        "internalType": "struct FrimaMarketplace.Item",
        "name": "",
        "type": "tuple"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "itemIdCounter",
    "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [{"internalType": "uint256", "name": "", "type": "uint256"}],
    "name": "items",
    "outputs": [
      {"internalType": "uint256", "name": "itemId", "type": "uint256"},
      {"internalType": "uint256", "name": "tokenId", "type": "uint256"},
      {"internalType": "string", "name": "title", "type": "string"},
      {"internalType": "uint256", "name": "price", "type": "uint256"},
      {"internalType": "string", "name": "explanation", "type": "string"},
      {"internalType": "string", "name": "imageUrl", "type": "string"},
      {"internalType": "string", "name": "uid", "type": "string"},
      {"internalType": "uint256", "name": "createdAt", "type": "uint256"},
      {"internalType": "uint256", "name": "updatedAt", "type": "uint256"},
      {"internalType": "bool", "name": "isPurchased", "type": "bool"},
      {"internalType": "string", "name": "category", "type": "string"},
      {"internalType": "address payable", "name": "seller", "type": "address"},
      {"internalType": "address payable", "name": "buyer", "type": "address"},
      {"internalType": "uint8", "name": "status", "type": "uint8"}
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {"internalType": "string", "name": "_title", "type": "string"},
      {"internalType": "uint256", "name": "_price", "type": "uint256"},
      {"internalType": "string", "name": "_explanation", "type": "string"},
      {"internalType": "string", "name": "_imageUrl", "type": "string"},
      {"internalType": "string", "name": "_uid", "type": "string"},
      {"internalType": "string", "name": "_category", "type": "string"},
      {"internalType": "string", "name": "_tokenURI", "type": "string"}
    ],
    "name": "listItem",
    "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "nftContract",
    "outputs": [{"internalType": "contract FrimaNFT", "name": "", "type": "address"}],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {"internalType": "uint256", "name": "_itemId", "type": "uint256"},
      {"internalType": "string", "name": "_title", "type": "string"},
      {"internalType": "uint256", "name": "_price", "type": "uint256"},
      {"internalType": "string", "name": "_explanation", "type": "string"},
      {"internalType": "string", "name": "_imageUrl", "type": "string"},
      {"internalType": "string", "name": "_category", "type": "string"}
    ],
    "name": "updateItem",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  }
]`
