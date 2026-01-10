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
)

type Player struct {
	ID        string  `json:"id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	LastShot  int64
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

	// ✅ SEND PLAYER ID TO CLIENT ON CONNECT
	c.WriteJSON(map[string]string{
		"id": id,
	})

	for {
		var msg map[string]float64
		if err := c.ReadJSON(&msg); err != nil {
			break
		}

		mu.Lock()
		p, ok := players[id]
		if !ok {
			mu.Unlock()
			continue
}


		// Move player once
		p.X += msg["dx"] * 5
		p.Y += msg["dy"] * 5

		// Clamp to large float64 bounds (full screen)
		maxX := 10000.0
		maxY := 10000.0
		if p.X < 0 {
			p.X = 0
		} else if p.X > maxX-PLAYER_SIZE {
			p.X = maxX - PLAYER_SIZE
		}
		if p.Y < 0 {
			p.Y = 0
		} else if p.Y > maxY-PLAYER_SIZE {
			p.Y = maxY - PLAYER_SIZE
		}

		// Shoot bullet
// Shoot bullet (1s cooldown)

now := time.Now().Unix()
if msg["shoot"] == 1 && now-p.LastShot >= 1 {
	p.LastShot = now
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
		hit := false
		for _, b := range bullets {
			b.X += b.DX
			b.Y += b.DY

			// Check collisions with players (except owner)
			for _, p := range players {
				if p.ID != b.O {
					dx := p.X - b.X
					dy := p.Y - b.Y
					dist := math.Sqrt(dx*dx + dy*dy)
					if dist < (PLAYER_SIZE+BULLET_SIZE)/2 {
						hit = true
						break
					}
				}
			}

			// Keep bullets in bounds
			if b.X >= 0 && b.Y >= 0 && b.X <= 10000 && b.Y <= 10000 {
				nb = append(nb, b)
			}
		}
		bullets = nb

		// Respawn all players if hit
		if hit {
			for _, p := range players {
				p.X = rand.Float64() * 600
				p.Y = rand.Float64() * 400
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
<body style="margin:0;overflow:hidden">
<canvas id="c"></canvas>
<script>
const ws = new WebSocket("wss://" + location.host + "/ws");
const c = document.getElementById("c");
const ctx = c.getContext("2d");

function resizeCanvas() {
  c.width = window.innerWidth;
  c.height = window.innerHeight;
}
resizeCanvas();
window.addEventListener("resize", resizeCanvas);

let keys = {};
let angle = 0;
let shoot = 0;

let myId = null;
let myPlayer = null;

document.onkeydown = e => keys[e.key] = true;
document.onkeyup   = e => keys[e.key] = false;

// ✅ Angle from PLAYER → MOUSE
document.onmousemove = e => {
  if (!myPlayer) return;

  const mx = e.clientX;
  const my = e.clientY;

  const px = myPlayer.x + 10;
  const py = myPlayer.y + 10;

  angle = Math.atan2(my - py, mx - px);
};

document.onclick = () => shoot = 1;

ws.onopen = () => {
  setInterval(() => {
    ws.send(JSON.stringify({
      dx: (keys.a ? -1 : 0) + (keys.d ? 1 : 0),
      dy: (keys.w ? -1 : 0) + (keys.s ? 1 : 0),
      a: angle,
      shoot: shoot
    }));
    shoot = 0;
  }, 16);
};

ws.onmessage = e => {
  const s = JSON.parse(e.data);

  // ✅ ID packet from server
  if (s.id) {
    myId = s.id;
    return;
  }

  if (!myId || !s.p[myId]) return;

  myPlayer = s.p[myId];

  ctx.fillStyle = "black";      // <-- set background color
  ctx.fillRect(0, 0, c.width, c.height);  // <-- fill the whole canvas


  // draw players
  for (const id in s.p) {
    ctx.fillStyle = "red";  // <-- player color
    ctx.fillRect(p.x, p.y, 40, 40);

  }

  // draw bullets
  for (const b of s.b) {
    ctx.fillRect(b.x, b.y, 6, 6);
  }
};
</script>


</body>
</html>
`


