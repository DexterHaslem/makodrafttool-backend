package main

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

const draftLogicRateMs = 750

var draftPhases = []phaseType{phaseTypeBan, phaseTypePick, phaseTypePick, phaseTypeBan, phaseTypePick}

// dont do anything clever to map sessions, they are looked up by all three ids
var sessions []*draft
var champs *Champions
var champsFlat []*Champion
var maps []*GameMap

var upgrader = websocket.Upgrader{
	ReadBufferSize:  128,
	WriteBufferSize: 128,
	CheckOrigin:     checkWebsocketOrigin,
}

func checkWebsocketOrigin(r *http.Request) bool {
	return true
}

func upgradeWs(ctx echo.Context) *websocket.Conn {
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

func createNewDraft(template *draftSetup) *draft {
	d := &draft{}
	d.wsWriteMutext = sync.Mutex{}
	d.IDs = &draftIDs{}
	d.Setup = template
	d.IDs.Admin = randomStr()
	d.IDs.Blue = randomStr()
	d.IDs.Red = randomStr()
	d.IDs.Results = randomStr()
	d.readonlyWss = make([]*websocket.Conn, 0)

	updateSnapshot(d)
	d.curSnapshot.DraftCreatedAt = time.Now()

	sessions = append(sessions, d)

	/* wait for admin to kick off draft, otherwise timers start */
	d.waitingStart = true
	return d
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
	/* TODO: any validation here */
	nd := createNewDraft(gameParams)

	go draftLogicLoop(nd)
	return c.JSONPretty(http.StatusOK, nd, "  ")
}

func draftLogicLoop(d *draft) {

	/* wait for draft to kick off */
	for d.waitingStart {
		time.Sleep(time.Millisecond * draftLogicRateMs)

		/* TODO: if someone creates a draft but never starts it, check here to nuke after a timeout */
		if d.curSnapshot == nil {
			continue
		}

		if d.curSnapshot.DraftDone {
			/* dont force d/c clients, let them view results */
			return
		}
	}

	log.Printf("admin started draft '%s'\n", d.Setup.Name)

	/* admin hit button, we've started drafting */
	for i, p := range draftPhases {
		log.Printf("draft '%s': phase# %d (%v) started\n", d.Setup.Name, i, p)

		setupNextVote(d, p)

		/* run draft phase timer server side. if both teams lock in , move on early */
		donePhase := false
		redVoted := false
		blueVoted := false

		d.curSnapshot.VoteUnlimitedTime = d.Setup.VotingSecs[i] == 0
		d.curSnapshot.VoteTimeLeft = float32(d.Setup.VotingSecs[i])

		for !donePhase {
			time.Sleep(draftLogicRateMs * time.Millisecond)

			if !d.curSnapshot.VotePaused {
				if !d.curSnapshot.VoteUnlimitedTime {
					d.curSnapshot.VoteTimeLeft -= float32(draftLogicRateMs) / 1000.0 // ms -> sec
					d.curSnapshot.VoteTimeLeftPretty = fmt.Sprintf("%d", int(d.curSnapshot.VoteTimeLeft))
				}

				if redVoted != d.curSnapshot.CurrentVote.RedVoted {
					//needFullSnap = true
					redVoted = true
				}
				if blueVoted != d.curSnapshot.CurrentVote.BlueVoted {
					//needFullSnap = true
					blueVoted = true
				}

				donePhase = (!d.curSnapshot.VoteUnlimitedTime && d.curSnapshot.VoteTimeLeft <= 0.01) || (blueVoted && redVoted)

				if !donePhase {
					//if needFullSnap {
					sendSnap(d)
					//} else {
					//	sendTimerUpdate(d)
					//}
				}
			}
		}

		/* save the phase that just finished */
		cvCopy := *d.curSnapshot.CurrentVote
		d.curSnapshot.VoteActive = false
		cvCopy.HasVoted = true
		d.curSnapshot.Phases = append(d.curSnapshot.Phases, &cvCopy)

		sendSnap(d)

		log.Printf("draft '%s': phase# %d (%v) done\n", d.Setup.Name, i, p)

		if i < len(draftPhases) {
			time.Sleep(time.Second * time.Duration(d.Setup.PhaseDelaySecs))
		}
	}

	log.Printf("draft '%s' is done!", d.Setup.Name)
	d.curSnapshot.DraftDone = true
	d.curSnapshot.DraftEndedAt = time.Now()

	/* final snap */
	go sendSnap(d)
}

func getAllConnected(d *draft) []*websocket.Conn {
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
	return wsconns
}

func sendTimerUpdate(d *draft) {
	wsconns := getAllConnected(d)
	d.wsWriteMutext.Lock()
	for _, ws := range wsconns {
		tm := WsMsgTimerOnly{
			Type:               WsMsgSnapshotTimerOnly,
			VoteTimeLeftPretty: d.curSnapshot.VoteTimeLeftPretty,
		}
		ws.WriteJSON(tm)
	}

	d.wsWriteMutext.Unlock()
}

/* something changed, send current draft state to all active connections */
func sendSnap(d *draft) {
	updateSnapshot(d)

	wsconns := getAllConnected(d)
	d.wsWriteMutext.Lock()
	for _, ws := range wsconns {
		/* dont send pending vote stuff to others that would leak picks early */
		ss := *d.curSnapshot
		if d.curSnapshot.VoteActive {
			cvc := *ss.CurrentVote
			ss.CurrentVote = &cvc

			/* TODO: if filtering is ever implemented server side, disable this code */
			ss.CurrentVote.ValidRedValues = nil
			ss.CurrentVote.ValidBlueValues = nil

			if ws == d.redWs {
				ss.CurrentVote.VoteBlueValue = ""
			} else if ws == d.blueWs {
				ss.CurrentVote.VoteRedValue = ""
			} else {
				// r/o observer, remove all pending vote info
				// well almost, need current phase type, so dont nuke
				ss.CurrentVote.VoteBlueValue = ""
				ss.CurrentVote.VoteRedValue = ""
			}
		}

		ws.WriteJSON(ss)
	}
	d.wsWriteMutext.Unlock()
}

func updateSnapshot(d *draft) {
	if d.curSnapshot == nil {
		d.curSnapshot = &WsMsg{}
		d.curSnapshot.Phases = make([]*phaseVote, 0)
		d.curSnapshot.Type = WsMsgSnapshot
	}

	d.curSnapshot.DraftStarted = !d.waitingStart
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

		handleClientMessage(d, st, m)
	}
}

func handleClientMessage(d *draft, st sesType, m WsMsg) {
	dirty := false

	switch m.Type {
	case WsClientReady:
		/* do not toggle, just let them set it */
		if st == blue && !d.curSnapshot.BlueReady {
			d.curSnapshot.BlueReady = true
			dirty = true
		} else if st == red && !d.curSnapshot.RedReady {
			d.curSnapshot.RedReady = true
			dirty = true
		}

		// in wait phase, start as soon as both captains ready
		if d.waitingStart && d.curSnapshot.BlueReady && d.curSnapshot.RedReady {
			d.waitingStart = false
		}

	case WsMsgVoteAction:
		if d.curSnapshot.VoteActive && m.CurrentVote != nil {
			if st == blue && !d.curSnapshot.CurrentVote.BlueVoted {
				//log.Printf("got a vote from blue: %s", m.CurrentVote.VoteBlueValue)
				dirty = true
				d.curSnapshot.CurrentVote.BlueVoted = true
				d.curSnapshot.CurrentVote.VoteBlueValue = m.CurrentVote.VoteBlueValue
			} else if st == red && !d.curSnapshot.CurrentVote.RedVoted {
				//log.Printf("got a vote from red: %s", m.CurrentVote.VoteRedValue)
				dirty = true
				d.curSnapshot.CurrentVote.RedVoted = true
				d.curSnapshot.CurrentVote.VoteRedValue = m.CurrentVote.VoteRedValue
			}
		}
	case WsStartVoting:
		// NOTE: this is removed on UI, drafting starts when both captains ready now
		// this removes need of admin
		if st == admin {
			dirty = true
			d.waitingStart = false
		}

	case WsMsgAdminPauseTimer:
		if st == admin && d.curSnapshot.CurrentVote != nil {
			dirty = true
			d.curSnapshot.VotePaused = !d.curSnapshot.VotePaused
		}
	case WsMsgAdminResetTimer:
		if st == admin && d.curSnapshot.CurrentVote != nil {
			dirty = true
			d.curSnapshot.VoteTimeLeft = float32(d.Setup.VotingSecs[d.curSnapshot.CurrentVote.PhaseNum])
			d.curSnapshot.VoteTimeLeftPretty = fmt.Sprintf("%d", int(d.curSnapshot.VoteTimeLeft))
		}
	case WsMsgAdminOverrideVote:
		if st == admin {
			dirty = true
			m.CurrentVote.AdminOverride = true
			/* just overwrite state with admin edit and propagate */
			d.curSnapshot.Phases[m.CurrentVote.PhaseNum] = m.CurrentVote
		}
	}

	if dirty {
		sendSnap(d)
	}
}

func setupNextVote(d *draft, pt phaseType) {
	/* vote is automatically saved at end of timer in logic loop, dont copy here */
	d.curSnapshot.VoteActive = true
	d.curSnapshot.RedReady = false
	d.curSnapshot.BlueReady = false

	/* if filter logic implemented, these need to be re-added */
	// rc, bc := validChampsForCurPhase(d)

	d.curSnapshot.CurrentVote = &phaseVote{
		PhaseType: pt,
		/* if filter logic implemented, these need to be re-added */
		ValidBlueValues: nil,
		ValidRedValues:  nil,
		PhaseNum:        d.curSnapshot.CurrentPhase,
	}

	// log.Printf("starting vote # %d type = %v\n", d.curSnapshot.CurrentVote.PhaseNum, d.curSnapshot.CurrentVote.PhaseType)
	d.curSnapshot.CurrentPhase++
	d.curSnapshot.VotingStartedAt = time.Now()
}

type phaseChampSelections struct {
	red  []string
	blue []string
}

func validChampsForCurPhase(d *draft) *phaseChampSelections {
	allChamps := make([]string, 0)

	for _, cx := range champsFlat {
		allChamps = append(allChamps, cx.Name)
	}

	/* first vote, nothing to filter yet */
	if d.curSnapshot.CurrentVote == nil || d.curSnapshot.CurrentPhase < 1 {
		return &phaseChampSelections{
			red:  allChamps,
			blue: allChamps,
		}
	}

	retRed := make([]string, 0)
	retBlue := make([]string, 0)

	isPickPhase := d.curSnapshot.CurrentVote.PhaseType == phaseTypePick

	for _, cn := range allChamps {
		/* note not orthogonal
		for each champ:
			pick phase:
				- if same team already picked, remove
				- if opposite team banned, remove
			ban phase:
				- if same team already banned, remove
		*/
		validRed := true
		validBlue := true

		for _, pv := range d.curSnapshot.Phases {
			isBan := pv.PhaseType == phaseTypeBan

			if pv.VoteBlueValue == cn {
				if isBan {
					validRed = false
					if validBlue {
						validBlue = isPickPhase
					}
				} else {
					validBlue = false
					if validRed {
						validRed = isPickPhase
					}
				}
			}

			if pv.VoteRedValue == cn {
				if isBan {
					validBlue = false
					if validRed {
						validRed = isPickPhase
					}
				} else {
					validRed = false
					if validBlue {
						validBlue = isPickPhase
					}
				}
			}
		}

		if validRed {
			retRed = append(retRed, cn)
		}
		if validBlue {
			retBlue = append(retBlue, cn)
		}
	}

	return &phaseChampSelections{
		red:  retRed,
		blue: retBlue,
	}
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
	d, st, err := findDraft(c)
	if err != nil {
		return err
	}

	// we got a good draft. connect ws
	newWs := upgradeWs(c)
	switch st {
	case admin:
		d.adminWs = newWs
	case red:
		d.redWs = newWs
	case blue:
		d.blueWs = newWs
	case results:
		d.readonlyWss = append(d.readonlyWss, newWs)
	}

	/* update any connected clients that may be waiting for draft to start. dont do this blocking */
	go sendSnap(d)

	go wsClientLoop(d, newWs, st)
	return c.NoContent(http.StatusSwitchingProtocols)
}

// Start fires up the battlerite system. cfgDir sets where to look for config jsons, conn is the echo connection string, eg ":1234"
func Start(cfgDir string, conn string) {
	e := echo.New()

	champFn := path.Join(cfgDir, "champs.json")
	tryChamps, err := ReadChampions(champFn)
	if err != nil {
		log.Fatalf("failed to load champions from %s\n", champFn)
	} else {
		champs = tryChamps
		champsFlat = make([]*Champion, 0)

		for _, cx := range [][]*Champion{champs.Melee, champs.Ranged, champs.Support} {
			for i := 0; i < len(cx); i++ {
				champsFlat = append(champsFlat, cx[i])
			}
		}
	}

	mapsFn := path.Join(cfgDir, "maps.json")
	tryMaps, err := ReadMaps(mapsFn)
	if err != nil {
		log.Fatalf("failed to load maps from %s\n", mapsFn)
	} else {
		maps = tryMaps
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
	e.GET("/maps", getMapsHandler)
	// create just one endpoint for ws, we can figure out what type of session it is by code, to make frontend easier
	e.GET("/ws/:id", wsHandler)
	e.GET("/draftState/:id", draftStateHandler)
	e.GET("/draftReport/:id", draftReportHandler)

	e.Static("/static", "static")
}

func draftReportHandler(c echo.Context) error {
	d, _, err := findDraft(c)
	if err != nil {
		return c.String(http.StatusBadRequest, "no draft found")
	}

	reportTxt := generateDraftReport(d)

	// echo has no way to cause attachment download with in memory data. very cool, do manually in header here
	c.Response().Header().Set("Content-Disposition", "attachment; filename=draft.txt")
	return c.Blob(http.StatusOK, "text/plain", []byte(reportTxt))
}

func getChampionsHandler(c echo.Context) error {
	if champs == nil {
		return c.String(http.StatusOK, "{}")
	}

	return c.JSON(http.StatusOK, champs)
}

func getMapsHandler(c echo.Context) error {
	if maps == nil {
		return c.String(http.StatusOK, "[]")
	}

	return c.JSON(http.StatusOK, maps)
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
		ViewerCode:  d.IDs.Results,
		Phases:      d.curSnapshot.Phases,
	}
	return ds
}
