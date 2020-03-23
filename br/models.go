package br

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"os"
	"sync"
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
	Name           string `json:"name"`
	MapName        string `json:"mapName"`
	BlueName       string `json:"blueName"`
	RedName        string `json:"redName"`
	VotingSecs     []int  `json:"votingSecs"`
	PhaseDelaySecs int    `json:"phaseDelaySecs"`
}

type draft struct {
	Setup *draftSetup `json:"setup"`
	IDs   *draftIDs   `json:"ids"`

	adminWs     *websocket.Conn
	redWs       *websocket.Conn
	blueWs      *websocket.Conn
	readonlyWss []*websocket.Conn

	wsWriteMutext sync.Mutex

	waitingStart bool
	curSnapshot  *WsMsg
}

type wsMsgType int

const (
	WsMsgSnapshot          wsMsgType = 1
	WsMsgVoteAction        wsMsgType = 2
	WsMsgClientClose       wsMsgType = 3
	WsServerClose          wsMsgType = 4
	WsClientReady          wsMsgType = 5
	WsStartVoting          wsMsgType = 6
	WsMsgSnapshotTimerOnly wsMsgType = 9

	WsMsgAdminPauseTimer   wsMsgType = 11
	WsMsgAdminResetTimer   wsMsgType = 12
	WsMsgAdminOverrideVote wsMsgType = 13
)

type draftState struct {
	SessionType sesType      `json:"sessionType"`
	Setup       *draftSetup  `json:"setup"`
	Phases      []*phaseVote `json:"phases"`
}

type phaseType string

const (
	phaseTypePick phaseType = "pick"
	phaseTypeBan  phaseType = "ban"
)

type phaseVote struct {
	PhaseType       phaseType `json:"phaseType"`
	HasVoted        bool      `json:"hasVoted"`
	PhaseNum        int       `json:"phaseNum"`
	RedVoted        bool      `json:"redHasVoted"`
	BlueVoted       bool      `json:"blueHasVoted"`
	ValidRedValues  []string  `json:"validRedValues"`
	ValidBlueValues []string  `json:"validBlueValues"`
	VoteRedValue    string    `json:"voteRedValue"`
	VoteBlueValue   string    `json:"voteBlueValue"`
	AdminOverride   bool      `json:"adminOverride"`
}

type WsMsg struct {
	Type wsMsgType `json:"msgType"`

	AdminConnected bool `json:"adminConnected"`
	RedConnected   bool `json:"redConnected"`
	BlueConnected  bool `json:"blueConnected"`
	ResultsViewers int  `json:"resultsViewers"`
	RedReady       bool `json:"redReady"`
	BlueReady      bool `json:"blueReady"`

	VoteActive         bool         `json:"voteActive"`
	VotePaused         bool         `json:"votePaused"`
	VoteTimeLeft       float32      `json:"voteTimeLeft"`
	CurrentPhase       int          `json:"currentPhase"`
	CurrentVote        *phaseVote   `json:"currentVote"`
	Phases             []*phaseVote `json:"phases"`
	DraftDone          bool         `json:"draftDone"`
	DraftStarted       bool         `json:"draftStarted"`
	DraftCreatedAt     time.Time    `json:"draftCreatedAt"`
	DraftStartedAt     time.Time    `json:"draftStartedAt"`
	DraftEndedAt       time.Time    `json:"draftEndedAt"`
	VotingStartedAt    time.Time
	VoteTimeLeftPretty string `json:"voteTimeLeftPretty"`
}

type WsMsgTimerOnly struct {
	Type               wsMsgType `json:"msgType"`
	VoteTimeLeftPretty string    `json:"voteTimeLeftPretty"`
}

type Champion struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Asset       string `json:"asset"`
}

type Champions struct {
	Melee   []*Champion `json:"melee"`
	Ranged  []*Champion `json:"ranged"`
	Support []*Champion `json:"support"`
}

func ReadChampions(fn string) (*Champions, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bytes, _ := ioutil.ReadAll(f)
	champions := &Champions{}
	json.Unmarshal(bytes, champions)

	return champions, nil
}

type GameMap struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Asset       string `json:"asset"`
}

func ReadMaps(fn string) ([]*GameMap, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bytes, _ := ioutil.ReadAll(f)
	var maps []*GameMap
	json.Unmarshal(bytes, &maps)

	return maps, nil
}
