package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

// Nastavení WebSocketu (aby se dalo připojit z webu)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	// 1. Získání portu a DB adresy od Renderu
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		// Připojení k DB (pokud existuje)
		db, err := sql.Open("postgres", dbURL)
		if err != nil {
			log.Println("Chyba DB:", err)
		} else {
			defer db.Close()
			// Vytvoření tabulky pro leaderboard, pokud neexistuje
			_, err = db.Exec(`CREATE TABLE IF NOT EXISTS leaderboard (id SERIAL PRIMARY KEY, name TEXT, wins INT)`)
			if err != nil {
				log.Println("Chyba vytváření tabulky:", err)
			}
		}
	}

	// 2. Obsluha statických souborů (index.html)
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// 3. WebSocket endpoint (tudy tečou data hry)
	http.HandleFunc("/ws", handleConnections)

	log.Printf("Pérák server běží na portu %s", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	for {
		// Čekáme na zprávu od hráče (pohyb, střelba)
		messageType, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		// Posíláme zprávu zpět (echo) - později sem dáme logiku duelu
		if err := ws.WriteMessage(messageType, msg); err != nil {
			break
		}
	}
}
