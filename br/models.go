package br

import (
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type sesID uuid.UUID

type sesType int

const (
	// admin session type
	admin sesType = 1
	// blue captain session type
	blue sesType = 2
	// red captain session type
	red sesType = 3
)

type gameSes struct {
	mapName  string
	blueName string
	redName  string

	// admin session id
	sesID        sesID
	redSesID     sesID
	blueSesID    sesID
	resultsSesID sesID

	adminWs *websocket.Conn
	redWs   *websocket.Conn
	blueWs  *websocket.Conn
	roWs    []*websocket.Conn
}

func startSes(mapName, blueName, redName string) *gameSes {
	ns := gameSes
	ns.mapName = mapName
	ns.blueName = blueName
	ns.redName = redName
	return &ns
}
