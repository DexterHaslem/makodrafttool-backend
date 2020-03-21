package br

import (
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
	Admin   string `json:"admin"`
	Red     string `json:"red"`
	Blue    string `json:"blue"`
	Results string `json:"results"`
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

type WsMessage struct {
	Type wsMsgType `json:"msgType"`
}

type WsMsgSessionType struct {
	WsMessage
	SessionType sesType `json:"sessionType"`
}

type WsMsgSnapshot struct {
	WsMessage
	Setup          *draftSetup `json:"setup"`
	AdminConnected bool        `json:"adminConnected"`
	RedConnected   bool        `json:"redConnected"`
	BlueConnected  bool        `json:"blueConnected"`
	ResultsViewers int         `json:"resultsViewers"`
}
