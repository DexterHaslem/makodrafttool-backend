package br

import (
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo"
)

var c *websocket.Conn
var clients = make(map[*websocket.Conn]bool) // connected clients
//var broadcast = make(chan Message)           // broadcast channel
var upgrader = websocket.Upgrader{}

// Start fires up the battlerite system. cfgDir sets where to look for config jsons, conn is the echo connection string, eg ":1234"
func Start(cfgDir string, conn string) {
	u, _ := uuid.NewRandom()
	e := echo.New()
	e.GET("/admin", adminHandler)
	e.GET("/admin/:id", adminSesHandler)
	e.GET("/blue/:id", blueSesHandler)
	e.GET("/red/:id", redSesHandler)
	e.Logger.Fatal(e.Start(conn))
}
