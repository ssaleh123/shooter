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
	TICK_RATE    = 60
	PLAYER_SIZE  = 20
	BULLET_SIZE  = 6
	BULLET_SPEED = 8
	COOLDOWN     = 1 * time.Second
	SCREEN_W     = 800
	SCREEN_H     = 600
)

type Player struct {
	ID         string    `json:"id"`
	X          float64   `json:"x"`
	Y          float64   `json:"y"`
	LastShoot  time.Time `json:"-"`
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
		X:  rand.Float64()*(SCREEN_W-PLAYER_SIZE),
		Y:  rand.Float64()*(SCREEN_H-PLAYER_SIZE),
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

		// Move player and clamp to screen
		p.X += msg["dx"] * 5
		p.Y += msg["dy"] * 5
		if p.X < 0 {
			p.X = 0
		} else if p.X > SCREEN_W-PLAYER_SIZE {
			p.X = SCREEN_W - PLAYER_SIZE
		}
		if p.Y < 0 {
			p.Y = 0
		} else if p.Y > SCREEN_H-PLAYER_SIZE {
			p.Y = SCREEN_H - PLAYER_SIZE
		}

		// Shoot with cooldown
		if msg["shoot"] == 1 && time.Since(p.LastShoot) >= COOLDOWN {
			angle := msg["a"]
			bullets = append(bullets, Bullet{
				X:  p.X + PLAYER_SIZE/2 - BULLET_SIZE/2,
				Y:  p.Y + PLAYER_SIZE/2 - BULLET_SIZE/2,
				DX: math.Cos(angle) * BULLET_SPEED,
				DY: math.Sin(angle) * BULLET_SPEED,
				O:  id,
			})
			p.LastShoot = time.Now()
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
			if b.X >= 0 && b.Y >= 0 && b.X <= SCREEN_W && b.Y <= SCREEN_H {
				nb = append(nb, b)
			}
		}
		bullets = nb

		// Check collisions
		for _, b := range bullets {
			for id, p := range players {
				if id == b.O {
					continue
				}
				if b.X+6 > p.X && b.X < p.X+PLAYER_SIZE &&
					b.Y+6 > p.Y && b.Y < p.Y+PLAYER_SIZE {
					// Respawn everyone
					for _, pl := range players {
						pl.X = rand.Float64()*(SCREEN_W-PLAYER_SIZE)
						pl.Y = rand.Float64()*(SCREEN_H-PLAYER_SIZE)
					}
					// Remove this bullet
					b.X = -1000
					b.Y = -1000
				}
			}
		}

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

const html = `
<!DOCTYPE html>
<html>
<body style="margin:0;overflow:hidden;background:#888">
<canvas id="c"></canvas>
<script>
const ws = new WebSocket("ws://" + location.host + "/ws");
const c = document.getElementById("c");
const ctx = c.getContext("2d");
c.width = 800; c.height = 600;

let keys = {}, angle = 0, shoot = 0;

document.onkeydown = e => keys[e.key.toLowerCase()] = true;
document.onkeyup = e => keys[e.key.toLowerCase()] = false;
document.onmousemove = e => {
  angle = Math.atan2(e.clientY - c.height/2, e.clientX - c.width/2);
};
document.onclick = () => shoot = 1;

ws.onopen = () => {
  setInterval(() => {
    ws.send(JSON.stringify({
      dx: (keys['a']?-1:0)+(keys['d']?1:0),
      dy: (keys['w']?-1:0)+(keys['s']?1:0),
      a: angle,
      shoot: shoot
    }));
    shoot = 0;
  }, 16);
};

ws.onmessage = e => {
  const s = JSON.parse(e.data);
  ctx.fillStyle = "#888";
  ctx.fillRect(0,0,c.width,c.height);
  ctx.fillStyle = "#fff";
  for (const id in s.p) {
    const p = s.p[id];
    ctx.fillRect(p.x,p.y,20,20);
  }
  ctx.fillStyle = "#f00";
  for (const b of s.b) {
    ctx.fillRect(b.x,b.y,6,6);
  }
};
</script>
</body>
</html>
`
