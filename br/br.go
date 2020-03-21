package br

import (
	"crypto/sha512"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

// dont do anything clever to map sessions, they are looked up by all three ids
var sessions []*draft

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
	return c.JSONPretty(http.StatusOK, nd, "  ")
}

func createWsSnapshot(d *draft) *WsMsg {
	ss := &WsMsg{
		Type:           WsMsgSnapshot,
		Setup:          d.Setup,
		AdminConnected: d.adminWs != nil,
		BlueConnected:  d.blueWs != nil,
		RedConnected:   d.redWs != nil,
		ResultsViewers: len(d.readonlyWss),
	}
	return ss
}

/* something changed, send current draft state to all active connections */
func sendSnap(d *draft, st sesType) {
	ss := createWsSnapshot(d)
	ss.SessionType = st
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

	for _, ws := range wsconns {
		ws.WriteJSON(ss)
	}
}

func wsClientLoop(d *draft, ws *websocket.Conn, st sesType) {
	for {
		m := WsMsg{}
		err := ws.ReadJSON(&m)
		if err != nil {
			notifyClientDc(d, ws, st)
			break
		}

		handleClientMessage(d, ws, st, m)
	}
}

func handleClientMessage(d *draft, ws *websocket.Conn, st sesType, m WsMsg) {

}

func notifyClientDc(d *draft, ws *websocket.Conn, st sesType) {

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

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		// AllowMethods:     []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE, echo.OPTIONS},
		// AllowCredentials: true,
	}))

	e.POST("/newdraft", adminNewDraftHandler)

	// create just one endpoint for ws, we can figure out what it is by code, to make frontend easier
	e.GET("/ws/:id", wsHandler)
	e.Logger.Fatal(e.Start(conn))
}
