package main

import (
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Struktura pro zprávu mezi hráči
type Message struct {
	Type string  `json:"type"` // "join", "update", "shoot", "hit"
	Room string  `json:"room"`
	X    float64 `json:"x,omitempty"`
	Y    float64 `json:"y,omitempty"`
	VX   float64 `json:"vx,omitempty"` // Rychlost střely X
	VY   float64 `json:"vy,omitempty"` // Rychlost střely Y
}

// Klient
type Client struct {
	Conn *websocket.Conn
	Room string
}

// Globální mapa místností: NazevMistnosti -> Seznam Hráčů
var lobbies = make(map[string]map[*Client]bool)
var mutex = sync.Mutex{}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleConnections)

	log.Printf("Pérák v2.0 běží na portu %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	client := &Client{Conn: ws}

	for {
		var msg Message
		// Čteme JSON zprávu od klienta
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("Hráč se odpojil: %v", err)
			leaveRoom(client)
			break
		}

		if msg.Type == "join" {
			// Hráč se chce připojit do místnosti
			joinRoom(client, msg.Room)
		} else {
			// Herní akce (pohyb, střelba) - rozešleme to soupeři v místnosti
			broadcastToRoom(client, msg)
		}
	}
}

func joinRoom(client *Client, roomName string) {
	mutex.Lock()
	defer mutex.Unlock()

	client.Room = roomName
	if lobbies[roomName] == nil {
		lobbies[roomName] = make(map[*Client]bool)
	}

	// Maximálně 2 hráči na místnost
	if len(lobbies[roomName]) >= 2 {
		client.Conn.WriteJSON(Message{Type: "error", Room: "Místnost je plná!"})
		return
	}

	lobbies[roomName][client] = true
	log.Printf("Hráč vstoupil do roomky: %s (Hráčů: %d)", roomName, len(lobbies[roomName]))

	// Pokud jsou tam 2, dáme vědět, že hra začíná
	if len(lobbies[roomName]) == 2 {
		msg := Message{Type: "start"}
		for c := range lobbies[roomName] {
			c.Conn.WriteJSON(msg)
		}
	}
}

func leaveRoom(client *Client) {
	mutex.Lock()
	defer mutex.Unlock()
	if client.Room != "" && lobbies[client.Room] != nil {
		delete(lobbies[client.Room], client)
		// Pokud místnost zůstala prázdná, smažeme ji
		if len(lobbies[client.Room]) == 0 {
			delete(lobbies, client.Room)
		} else {
			// Pokud tam někdo zbyl, řekneme mu, že vyhrál kontumací
			for c := range lobbies[client.Room] {
				c.Conn.WriteJSON(Message{Type: "win_disconnect"})
			}
		}
	}
}

func broadcastToRoom(sender *Client, msg Message) {
	mutex.Lock()
	defer mutex.Unlock()
	roomClients := lobbies[sender.Room]
	for client := range roomClients {
		// Posíláme zprávu jen SOUPEŘI (ne sobě)
		if client != sender {
			client.Conn.WriteJSON(msg)
		}
	}
}
