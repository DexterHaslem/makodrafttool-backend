package br

import (
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type sesType int

const (
	// admin session type
	admin sesType = 1
	// blue captain session type
	blue sesType = 2
	// red captain session type
	red sesType = 3
	//
	results sesType = 4
)

type gameMap struct {
}

type gameCharacter struct {
}

// break draft ids out so we can selectively marshal over ws
type draftIDs struct {
	Admin   uuid.UUID `json:"admin"`
	Red     uuid.UUID `json:"red"`
	Blue    uuid.UUID `json:"blue"`
	Results uuid.UUID `json:"results"`
}

// draft info all clients care about
type draftSetup struct {
	Name          string `json:"name"`
	MapName       string `json:"mapName"`
	BlueName      string `json:"blueName"`
	RedName       string `json:"redName"`
	VoteSecs      int    `json:"voteSecs"`
	CountdownSecs int    `json:"countdownSecs"`
}

type draft struct {
	Setup *draftSetup `json:"setup"`
	IDs   *draftIDs   `json:"ids"`

	adminWs     *websocket.Conn
	redWs       *websocket.Conn
	blueWs      *websocket.Conn
	readonlyWss []*websocket.Conn
}

type wsMsgType int

const (
	WS_MSG_SNAPSHOT     wsMsgType = 1
	WS_MSG_SESSION_TYPE           = 2
	WS_MSG_VOTE_ACTION            = 3
)

type wsMessage struct {
	mType wsMsgType `json:"msgType"`
}

type wsMsgSessionType struct {
	wsMessage
	sessionType sesType `json:"sessionType"`
}

type wsMsgSnapshot struct {
	wsMessage
	setup          *draftSetup `json:"setup"`
	adminConnected bool        `json:"adminConnected"`
	redConnected   bool        `json:"redConnected"`
	blueConnected  bool        `json:"blueConnected"`
	resultsViewers int         `json:"resultsViewers"`
}
