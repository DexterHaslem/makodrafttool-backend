package br

import (
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

// dont do anything clever to map sessions, they are looked up by all three ids
var sessions []*draft
var upgrader = websocket.Upgrader{}

func setAdminWs(s *draft, ctx echo.Context) error {
	return nil
}

func setBlueWs(s *draft, ctx echo.Context) error {

	return nil
}

func setRedWs(s *draft, ctx echo.Context) error {

	return nil
}

func addReadOnlyWs(s *draft, ctx echo.Context) error {

	return nil
}

func upgradeWS(ctx echo.Context) *websocket.Conn {
	ws, err := upgrader.Upgrade(ctx.Response(), ctx.Request(), nil)
	if err != nil {
		log.Fatal(err)
	}
	return ws
}

func startSes(template *draft) *draft {
	ns := *template
	ns.IDs = &draftIDs{}
	ns.IDs.Admin, _ = uuid.NewRandom()
	ns.IDs.Blue, _ = uuid.NewRandom()
	ns.IDs.Red, _ = uuid.NewRandom()
	ns.IDs.Results, _ = uuid.NewRandom()
	return &ns
}

func findDraft(ctx echo.Context, st sesType) (*draft, sesType, error) {
	id := ctx.Param("id")

	for _, s := range sessions {
		var sid uuid.UUID
		var st sesType
		switch st {
		case admin:
			sid = s.IDs.Admin
			st = admin
		case blue:
			sid = s.IDs.Blue
			st = blue
		case red:
			sid = s.IDs.Red
			st = red
		case results:
			sid = s.IDs.Results
			st = results
		}

		if sid.String() == id {
			return s, st, nil
		}
	}
	return nil, 0, ctx.String(http.StatusBadRequest, "no session")
}

func adminNewDraftHandler(c echo.Context) error {
	gameParams := &draft{}
	c.Bind(gameParams)
	nd := startSes(gameParams)
	return c.JSONPretty(http.StatusOK, nd, "  ")
}

func createWsSnapshot(d *draft) *wsMsgSnapshot {
	ss := &wsMsgSnapshot{
		wsMessage:      wsMessage{mType: WS_MSG_SNAPSHOT},
		setup:          d.Setup,
		adminConnected: d.adminWs != nil,
		blueConnected:  d.blueWs != nil,
		redConnected:   d.redWs != nil,
		resultsViewers: len(d.readonlyWss),
	}
	return ss
}

func sendSesType(ws *websocket.Conn, st sesType) {
	m := &wsMsgSessionType{
		wsMessage:   wsMessage{mType: WS_MSG_SESSION_TYPE},
		sessionType: st,
	}

	ws.WriteJSON(m)
}

/* something changed, send current draft state to all active connections */
func sendSnap(d *draft) {
	ss := createWsSnapshot(d)
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

func wsHandler(c echo.Context) error {
	s, st, err := findDraft(c, admin)
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

	defer sendSesType(newWs, st)
	defer sendSnap(s)
	return c.NoContent(http.StatusSwitchingProtocols)
}

// Start fires up the battlerite system. cfgDir sets where to look for config jsons, conn is the echo connection string, eg ":1234"
func Start(cfgDir string, conn string) {
	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
	}))

	// e.GET("/admin", adminHandler)
	e.POST("/newdraft", adminNewDraftHandler)

	// create just one endpoint for ws, we can figure out what it is by code, to make frontend easier
	e.GET("/ws/:id", wsHandler)
	e.Logger.Fatal(e.Start(conn))
}
