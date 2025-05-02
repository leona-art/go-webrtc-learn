package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// 接続を許可するオリジンを指定
	// localhost:3000からの接続を許可
	CheckOrigin: func(r *http.Request) bool {

		return true
	},
}

// クライアントの接続を管理するマップ
var (
	clients     = make(map[*websocket.Conn]bool)
	clientMutex sync.Mutex
)

// メッセージを送信するチャネル
var broadcast = make(chan Message)

// クライアント間で交換するメッセージの構造体
type Message struct {
	Type      string      `json:"type"`                // "offer", "answer", "candidate", "chat", etc.
	Payload   interface{} `json:"payload"`             // SDPまたはICE Candidateのデータなど
	Sender    string      `json:"sender,omitempty"`    // 送信者ID (あれば)
	Recipient string      `json:"recipient,omitempty"` // 受信者ID (あれば)
}

func handleConnection(w http.ResponseWriter, r *http.Request) {
	log.Println("新しい接続を受け付けています...")
	// WebSocket接続をアップグレード
	ws, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Printf("接続のアップグレードに失敗しました: %v", err)
		return
	}
	defer ws.Close()

	// クライアントを追加
	clientMutex.Lock()
	clients[ws] = true
	clientMutex.Unlock()
	log.Println("新しいクライアントが接続しました")

	// クライアントからのメッセージを受信
	for {
		var msg Message
		// JSONとしてメッセージを読み込む
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("エラーが発生しました: %v", err)
			// エラーが発生した場合はクライアントを削除
			clientMutex.Lock()
			delete(clients, ws)
			clientMutex.Unlock()
			break
		}
		msg.Sender = ws.RemoteAddr().String() // 送信者のアドレスを設定
		// メッセージをブロードキャストに送信
		broadcast <- msg
	}
}

func handleMessages() {
	for {
		// ブロードキャストチャネルからメッセージを受信
		msg := <-broadcast
		deleteClients := []*websocket.Conn{}
		for client := range clients {
			if msg.Sender == client.RemoteAddr().String() {
				// 自分に送信するメッセージはスキップ
				continue
			}
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("メッセージの送信に失敗しました: %v", err)
				client.Close()
				// エラーが発生した場合はクライアントを削除
				deleteClients = append(deleteClients, client)
			} else {
				log.Printf("メッセージをクライアントに送信しました: %v", msg)
				log.Printf("クライアントのID: %v", client.RemoteAddr())
			}
		}
		clientMutex.Lock()
		for _, client := range deleteClients {
			delete(clients, client)
		}
		clientMutex.Unlock()
		deleteClients = nil
	}
}

func main() {
	// メッセージ処理を行うゴルーチンを起動
	go handleMessages()

	// WebSocket接続を受け付けるハンドラを設定
	http.HandleFunc("/ws", handleConnection)

	ipAddr := "0.0.0.0"
	port := ":8080"
	// サーバーを起動
	log.Printf("サーバーを%sで起動します...", ipAddr+port)
	err := http.ListenAndServe(ipAddr+port, nil)
	if err != nil {
		log.Fatal("サーバーの起動に失敗しました:", err)
	}
	log.Println("サーバーが停止しました")
}
