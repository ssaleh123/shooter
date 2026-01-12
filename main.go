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
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	LastShot int64
	Kills    int     `json:"Kills"`
	Deaths   int     `json:"Deaths"`
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

	// first message MUST contain username
	var join struct {
		Name string `json:"name"`
	}
	if err := c.ReadJSON(&join); err != nil || join.Name == "" {
		c.Close()
		return
	}

	id := randString(8)

	mu.Lock()
	players[id] = &Player{
		ID:   id,
		Name: join.Name,
		X:    rand.Float64() * 1340,
		Y:    rand.Float64() * 730,
	}
	conns[id] = c
	mu.Unlock()

	c.WriteJSON(map[string]string{
		"id": id,
	})

	for {
		var msg map[string]float64
		if err := c.ReadJSON(&msg); err != nil {
			break
		}

		mu.Lock()
		p := players[id]

		p.X += msg["dx"] * 5
		p.Y += msg["dy"] * 5

		p.X = math.Max(0, math.Min(p.X, 1340-PLAYER_SIZE))
		p.Y = math.Max(0, math.Min(p.Y, 730-PLAYER_SIZE))


		now := time.Now().Unix()
		if msg["shoot"] == 1 && now-p.LastShot >= 1 {
			p.LastShot = now
			a := msg["a"]
			bullets = append(bullets, Bullet{
				X:  p.X,
				Y:  p.Y,
				DX: math.Cos(a) * BULLET_SPEED,
				DY: math.Sin(a) * BULLET_SPEED,
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
	t := time.NewTicker(time.Second / TICK_RATE)
	for range t.C {
		mu.Lock()

		nb := bullets[:0]

		for _, b := range bullets {
			b.X += b.DX
			b.Y += b.DY

			hit := false

			for _, p := range players {
				if p.ID == b.O {
					continue
				}

				// Check if bullet overlaps the square (anywhere)
if b.X+6 > p.X && b.X < p.X+PLAYER_SIZE &&
   b.Y+6 > p.Y && b.Y < p.Y+PLAYER_SIZE {
	// respawn hit player
	// respawn hit player and update kills/deaths
p.X = rand.Float64() * 1340
p.Y = rand.Float64() * 730
p.Deaths += 1                // hit player dies
if shooter, ok := players[b.O]; ok {
    shooter.Kills += 1       // shooter gets a kill
}
hit = true
break

}

			}

			// keep bullet if no hit and in bounds
			if !hit && b.X >= 0 && b.Y >= 0 &&
   b.X <= 1340 && b.Y <= 730 {
	nb = append(nb, b)
}

		}

		bullets = nb

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
<body style="margin:0;overflow:hidden;background:black">

<div id="menu" style="
	position:absolute;
	inset:0;
	display:flex;
	justify-content:center;
	align-items:center;
	background:black;
	z-index:10;
">
	<div>
		<input id="name" placeholder="Username"
			style="font-size:20px;padding:6px" maxlength="10" />
		<button onclick="start()"
			style="font-size:20px;padding:6px">Play</button>
	</div>
</div>

<canvas id="c"></canvas>

<script>
const c = document.getElementById("c");
const ctx = c.getContext("2d");

function resize() {
	c.width = innerWidth;
	c.height = innerHeight;
}
resize();
onresize = resize;

let ws, myId, myPlayer;
let keys = {}, angle = 0, shoot = 0;

function start() {
	const name = document.getElementById("name").value.trim();
	if (!name) return;

	document.getElementById("menu").style.display = "none";

	ws = new WebSocket("wss://" + location.host + "/ws");

	ws.onopen = () => {
		ws.send(JSON.stringify({ name }));
		setInterval(sendInput, 16);
	};

	ws.onmessage = e => render(JSON.parse(e.data));
}
document.getElementById("name").onkeydown = e => {
	if (e.key === "Enter") start();
};
function sendInput() {
	ws.send(JSON.stringify({
		dx: (keys.a?-1:0)+(keys.d?1:0),
		dy: (keys.w?-1:0)+(keys.s?1:0),
		a: angle,
		shoot
	}));
	shoot = 0;
}

document.onkeydown = e => keys[e.key] = true;
document.onkeyup = e => keys[e.key] = false;
onclick = () => shoot = 1;

onmousemove = e => {
	if (!myPlayer) return;
	angle = Math.atan2(
		e.clientY - (myPlayer.y+10),
		e.clientX - (myPlayer.x+10)
	);
};

function render(s) {
	if (s.id) { myId = s.id; return; }
	if (!s.p[myId]) return;

	myPlayer = s.p[myId];

	ctx.fillStyle = "black";
	ctx.fillRect(0,0,c.width,c.height);

	// draw players
	for (const id in s.p) {
		const p = s.p[id];

		ctx.fillStyle = "white";
		ctx.fillRect(p.x, p.y, 20, 20);

		ctx.fillStyle = "red";
		ctx.font = "18px sans-serif";
		ctx.textAlign = "center";
		ctx.fillText(p.name, p.x + 10, p.y - 5);
	}

	// draw bullets
	ctx.fillStyle = "white";
	for (const b of s.b) {
		ctx.fillRect(b.x, b.y, 6, 6);
	}

	// draw scoreboard on bottom right
	const rows = Object.values(s.p);
	const maxRows = 10;
	const rowHeight = 25;
	const colWidth = 50;
	const startX = 1340 + 20; // right side of map
	const startY = 730 - (Math.min(rows.length, maxRows) * rowHeight) - 20;

	ctx.fillStyle = "white";
	ctx.font = "16px sans-serif";
	ctx.textAlign = "left";

	for (let i = 0; i < rows.length && i < maxRows; i++) {
		const player = rows[i];
		const y = startY + i * rowHeight;
		ctx.fillText(player.Name, startX, y);
		ctx.fillText("K: " + (player.Kills || 0), startX + 100, y);
		ctx.fillText("D: " + (player.Deaths || 0), startX + 160, y);
	}
}

</script>
</body>
</html>
`

















