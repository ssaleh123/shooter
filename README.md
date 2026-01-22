Go WebSocket Shooter

A simple real-time multiplayer browser shooter built with Go + WebSockets.
Players move, aim with the mouse, shoot colored bullets, and track kills/deaths live.

Features

Real-time multiplayer using WebSockets

WASD movement + mouse aiming

Shooting with cooldown

Kill / death tracking & live scoreboard

Player color selection

Single-file Go server with embedded HTML client

Tech Stack

Go

Gorilla WebSocket

HTML5 Canvas (no external frontend libs)

Run Locally
go run main.go


Then open:

http://localhost:10000



Controls

WASD – Move

Mouse – Aim

Click – Shoot

Notes

Server runs at 60 ticks per second

