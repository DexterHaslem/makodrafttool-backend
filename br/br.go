package br

import (
	"crypto/sha512"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

const SnapshotSyncMs = 800

// dont do anything clever to map sessions, they are looked up by all three ids
var sessions []*draft
var champs *Champions

var upgrader = websocket.Upgrader{
	ReadBufferSize:  256,
	WriteBufferSize: 256,
	CheckOrigin:     checkWebsocketOrigin,
}

func checkWebsocketOrigin(r *http.Request) bool {
	return true
}

func upgradeWS(ctx echo.Context) *websocket.Conn {
	ws, err := upgrader.Upgrade(ctx.Response(), ctx.Request(), nil)
	if err != nil {
		log.Fatal(err)
	}
	return ws
}

func init() {
	rand.Seed(time.Now().Unix())
}

func randomStr() string {
	r := rand.Int()
	s := fmt.Sprintf("%d %d", r, time.Now().UnixNano())
	cf := sha512.New()
	cf.Write([]byte(s))
	h := cf.Sum(nil)
	cf.Reset()
	randStr := fmt.Sprintf("%x", h)
	return randStr[0:16]
}

func startDraft(template *draftSetup) *draft {
	ns := &draft{}
	ns.wsWriteMutext = sync.Mutex{}
	ns.IDs = &draftIDs{}
	ns.Setup = template
	ns.IDs.Admin = randomStr()
	ns.IDs.Blue = randomStr()
	ns.IDs.Red = randomStr()
	ns.IDs.Results = randomStr()
	ns.readonlyWss = make([]*websocket.Conn, 0)
	sessions = append(sessions, ns)

	return ns
}

func findDraft(ctx echo.Context) (*draft, sesType, error) {
	id := ctx.Param("id")

	// a lil nasty, figure out a better way to tag session code types
	for _, s := range sessions {
		if s.IDs.Admin == id {
			return s, admin, nil
		}
		if s.IDs.Results == id {
			return s, results, nil
		}
		if s.IDs.Red == id {
			return s, red, nil
		}
		if s.IDs.Blue == id {
			return s, blue, nil
		}
	}
	return nil, 0, ctx.String(http.StatusBadRequest, "no session")
}

func adminNewDraftHandler(c echo.Context) error {
	gameParams := &draftSetup{}
	c.Bind(gameParams)
	nd := startDraft(gameParams)

	go draftLogicLoop(nd)
	return c.JSONPretty(http.StatusOK, nd, "  ")
}

func draftLogicLoop(d *draft) {
	for {
		time.Sleep(time.Millisecond * SnapshotSyncMs)

		if d.curSnapshot != nil && d.curSnapshot.VoteActive {
			voteTime := time.Now().Sub(d.curSnapshot.VotingStartedAt)
			if int(voteTime.Seconds()) >= d.Setup.VoteSecs {
				d.curSnapshot.VoteActive = false
				/* TODO: transistion */
			}
		}
		sendSnap(d)
	}
}

/* something changed, send current draft state to all active connections */
func sendSnap(d *draft) {
	updateSnapshot(d)

	wsconns := make([]*websocket.Conn, 0)
	if d.adminWs != nil {
		wsconns = append(wsconns, d.adminWs)
	}
	if d.redWs != nil {
		wsconns = append(wsconns, d.redWs)
	}
	if d.blueWs != nil {
		wsconns = append(wsconns, d.blueWs)
	}

	if d.readonlyWss != nil {
		for _, roWs := range d.readonlyWss {
			wsconns = append(wsconns, roWs)
		}
	}

	d.wsWriteMutext.Lock()
	for _, ws := range wsconns {
		/* dont send pending vote stuff to others that would leak picks early */
		ss := *d.curSnapshot
		if d.curSnapshot.VoteActive {
			cvc := *ss.CurrentVote
			ss.CurrentVote = &cvc
			if ws != d.adminWs {
				if ws == d.redWs {
					ss.CurrentVote.VoteBlueValue = ""
					ss.CurrentVote.ValidBlueValues = nil
				} else if ws == d.blueWs {

					ss.CurrentVote.VoteRedValue = ""
					ss.CurrentVote.ValidRedValues = nil
				} else {
					// r/o observer, remove all pending vote info
					ss.CurrentVote = nil
				}
			}
		}
		ws.WriteJSON(ss)
	}
	d.wsWriteMutext.Unlock()
}

func updateSnapshot(d *draft) {
	if d.curSnapshot == nil {
		d.curSnapshot = &WsMsg{}
	}

	d.curSnapshot.Type = WsMsgSnapshot
	d.curSnapshot.AdminConnected = d.adminWs != nil
	d.curSnapshot.BlueConnected = d.blueWs != nil
	d.curSnapshot.RedConnected = d.redWs != nil
	d.curSnapshot.ResultsViewers = len(d.readonlyWss)
}

func wsClientLoop(d *draft, ws *websocket.Conn, st sesType) {
	for {
		m := WsMsg{}
		err := ws.ReadJSON(&m)
		if err != nil {
			notifyClientDc(d, ws)
			ws.Close()
			break
		}

		handleClientMessage(d, ws, st, m)
	}
}

func handleClientMessage(d *draft, ws *websocket.Conn, st sesType, m WsMsg) {
	switch m.Type {
	case WsClientReady:
		if ws == d.blueWs {
			d.curSnapshot.BlueReady = !d.curSnapshot.BlueReady
		} else if ws == d.redWs {
			d.curSnapshot.RedReady = !d.curSnapshot.RedReady
		}
	case WsMsgVoteAction:
		if d.curSnapshot.VoteActive {
			if m.CurrentVote != nil {
				if ws == d.blueWs {
					d.curSnapshot.CurrentVote.BlueVoted = true
					d.curSnapshot.CurrentVote.VoteBlueValue = m.CurrentVote.VoteBlueValue
				} else if ws == d.redWs {
					d.curSnapshot.CurrentVote.RedVoted = true
					d.curSnapshot.CurrentVote.VoteRedValue = m.CurrentVote.VoteRedValue
				}
			}
		}
	case WsStartVoting:
		if ws == d.adminWs {
			/* send notification to two captains to start countdown + pick timer */
			setupNextVote(d)
		}
	}

	sendSnap(d)
}

func setupNextVote(d *draft) {
	/* TODO: read phase from phases */

	d.curSnapshot.VoteActive = true
	d.curSnapshot.CurrentPhase++
	allChamps := make([]string, 0)

	/* TODO: filter prev pick/bans */
	for _, cx := range [][]*Champion{champs.Melee, champs.Ranged, champs.Support} {
		for i := 0; i < len(cx); i++ {
			allChamps = append(allChamps, cx[i].Name)
		}
	}

	d.curSnapshot.CurrentVote = &phaseVote{
		PhaseType:       phaseTypePick,
		ValidBlueValues: allChamps,
		ValidRedValues:  allChamps,
		PhaseNum:        d.curSnapshot.CurrentPhase,
	}
	d.curSnapshot.VotingStartedAt = time.Now()
}

func notifyClientDc(d *draft, ws *websocket.Conn) {
	if d == nil || ws == nil {
		return
	}

	// connection of st was disconnected
	if d.adminWs == ws {
		d.adminWs = nil
	} else if d.blueWs == ws {
		d.blueWs = nil
	} else if d.redWs == ws {
		d.redWs = nil
	} else {
		// assume was spec, remove from list
		idx := -1
		for i := 0; i < len(d.readonlyWss); i++ {
			if d.readonlyWss[i] == ws {
				idx = i
				break
			}
		}
		if idx != -1 {
			d.readonlyWss[idx] = d.readonlyWss[len(d.readonlyWss)-1]
			d.readonlyWss = d.readonlyWss[:len(d.readonlyWss)-1]
		}
	}

	sendSnap(d)
}

func wsHandler(c echo.Context) error {
	s, st, err := findDraft(c)
	if err != nil {
		return err
	}

	// we got a good draft. connect ws
	newWs := upgradeWS(c)
	switch st {
	case admin:
		s.adminWs = newWs
	case red:
		s.redWs = newWs
	case blue:
		s.blueWs = newWs
	case results:
		s.readonlyWss = append(s.readonlyWss, newWs)
	}

	go wsClientLoop(s, newWs, st)
	return c.NoContent(http.StatusSwitchingProtocols)
}

// Start fires up the battlerite system. cfgDir sets where to look for config jsons, conn is the echo connection string, eg ":1234"
func Start(cfgDir string, conn string) {
	e := echo.New()

	champFn := path.Join(cfgDir, "champs.json")
	tryChamps, err := ReadChampions(champFn)
	if err != nil {
		fmt.Printf("failed to load champions from %s\n", champFn)
	} else {
		champs = tryChamps
	}

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		// AllowMethods:     []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE, echo.OPTIONS},
		// AllowCredentials: true,
	}))

	setupEndpoints(e)

	e.Logger.Fatal(e.Start(conn))
}

func setupEndpoints(e *echo.Echo) {
	e.POST("/newdraft", adminNewDraftHandler)
	e.GET("/champions", getChampionsHandler)
	// create just one endpoint for ws, we can figure out what it is by code, to make frontend easier
	e.GET("/ws/:id", wsHandler)
	e.GET("/draftState/:id", draftStateHandler)
}

func getChampionsHandler(c echo.Context) error {
	if champs == nil {
		return c.String(http.StatusOK, "")
	}

	return c.JSON(http.StatusOK, champs)
}

func draftStateHandler(c echo.Context) error {
	d, st, err := findDraft(c)
	if err != nil {
		return err
	}

	draftState := getDraftState(d, st)
	return c.JSON(http.StatusOK, draftState)
}

func getDraftState(d *draft, st sesType) *draftState {
	ds := &draftState{
		SessionType: st,
		Setup:       d.Setup,
		Phases:      nil, /* TODO */
	}
	return ds
}
