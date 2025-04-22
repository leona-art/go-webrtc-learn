package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// 接続を許可するオリジンを指定
	// localhost:3000からの接続を許可
	CheckOrigin: func(r *http.Request) bool {
		if r.Header.Get("Origin") == "http://localhost:3000" {
			return true
		}
		return false
	},
}

// クライアントの接続を管理するマップ
var clients = make(map[*websocket.Conn]bool)

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
	// WebSocket接続をアップグレード
	ws, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Printf("接続のアップグレードに失敗しました: %v", err)
		return
	}
	defer ws.Close()

	// クライアントを追加
	clients[ws] = true

	// クライアントからのメッセージを受信
	for {
		var msg Message
		// JSONとしてメッセージを読み込む
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("エラーが発生しました: %v", err)
			// エラーが発生した場合はクライアントを削除
			delete(clients, ws)
			break
		}
		// メッセージをブロードキャストに送信
		broadcast <- msg
	}
}

func handleMessages() {
	for {
		// ブロードキャストチャネルからメッセージを受信
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("メッセージの送信に失敗しました: %v", err)
				client.Close()
				// エラーが発生した場合はクライアントを削除
				delete(clients, client)
			}
		}
	}
}

func main() {
	// メッセージ処理を行うゴルーチンを起動
	go handleMessages()

	// WebSocket接続を受け付けるハンドラを設定
	http.HandleFunc("/ws", handleConnection)

	port := ":8080"
	// サーバーを起動
	log.Printf("サーバーをポート%sで起動します...", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("サーバーの起動に失敗しました:", err)
	}
	log.Println("サーバーが停止しました")
}
