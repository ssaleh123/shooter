package main

import (
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	TICK_RATE   = 60
	PLAYER_SIZE = 20
	BULLET_SIZE = 6
	BULLET_SPEED = 8
)

type Player struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

type Bullet struct {
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	DX float64 `json:"dx"`
	DY float64 `json:"dy"`
	O  string  `json:"o"`
}

var (
	players = map[string]*Player{}
	bullets = []Bullet{}
	conns   = map[string]*websocket.Conn{}
	mu      sync.Mutex
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	rand.Seed(time.Now().UnixNano())

	http.HandleFunc("/", serveHTML)
	http.HandleFunc("/ws", wsHandler)

	go gameLoop()

	log.Println("Server running on :10000")
	log.Fatal(http.ListenAndServe(":10000", nil))
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(html))
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	id := randString(8)

	mu.Lock()
	players[id] = &Player{
		ID: id,
		X:  rand.Float64() * 600,
		Y:  rand.Float64() * 400,
	}
	conns[id] = c
	mu.Unlock()

	for {
		var msg map[string]float64
		if err := c.ReadJSON(&msg); err != nil {
			break
		}

		mu.Lock()
		p := players[id]
		p.X += msg["dx"] * 5
		p.Y += msg["dy"] * 5

		if msg["shoot"] == 1 {
			angle := msg["a"]
			bullets = append(bullets, Bullet{
				X:  p.X,
				Y:  p.Y,
				DX: math.Cos(angle) * BULLET_SPEED,
				DY: math.Sin(angle) * BULLET_SPEED,
				O:  id,
			})
		}
		mu.Unlock()
	}

	mu.Lock()
	delete(players, id)
	delete(conns, id)
	mu.Unlock()
}

func gameLoop() {
	ticker := time.NewTicker(time.Second / TICK_RATE)
	for range ticker.C {
		mu.Lock()

		// Move bullets
		nb := bullets[:0]
		for _, b := range bullets {
			b.X += b.DX
			b.Y += b.DY
			if b.X >= 0 && b.Y >= 0 && b.X <= 800 && b.Y <= 600 {
				nb = append(nb, b)
			}
		}
		bullets = nb

		// Broadcast state
		state := map[string]interface{}{
			"p": players,
			"b": bullets,
		}

		for _, c := range conns {
			c.WriteJSON(state)
		}

		mu.Unlock()
	}
}

func randString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
