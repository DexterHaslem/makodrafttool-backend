package br

import (
	"github.com/gorilla/websocket"
	"time"
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
	startedAt time.Time

	Setup *draftSetup `json:"setup"`
	IDs   *draftIDs   `json:"ids"`

	adminWs     *websocket.Conn
	redWs       *websocket.Conn
	blueWs      *websocket.Conn
	readonlyWss []*websocket.Conn
}

type wsMsgType int

const (
	WsMsgSnapshot wsMsgType = 1
	WsMsgVoteAction
	WsMsgClientClose
	WsServerClose
	WsClientReady
)

type PhaseVote struct {
	HasVoted        bool     `json:"hasVoted"`
	PhaseNum        int      `json:"phaseNum"`
	ValidRedValues  []string `json:"validRedValues"`
	ValidBlueValues []string `json:"validBlueValues"`
	VoteRedValue    string   `json:"voteRedValue"`
	VoteBlueValue   string   `json:"voteBlueValue"`
}

type WsMsg struct {
	Type           wsMsgType   `json:"msgType"`
	SessionType    sesType     `json:"sessionType"`
	Setup          *draftSetup `json:"setup"`
	AdminConnected bool        `json:"adminConnected"`
	RedConnected   bool        `json:"redConnected"`
	BlueConnected  bool        `json:"blueConnected"`
	ResultsViewers int         `json:"resultsViewers"`
	RedReady       bool        `json:"blueReady"`
	BlueReady      bool        `json:"redReady"`

	CurrentPhase int         `json:"currentPhase"`
	Phases       []PhaseVote `json:"phases"`
}
