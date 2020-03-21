package br

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo"
)

// dont do anything clever to map sessions, they are looked up by all three ids
var sessions []*gameSes
var upgrader = websocket.Upgrader{}

func setAdminWs(s *gameSes, ctx echo.Context) error {

}

func setBlueWs(s *gameSes, ctx echo.Context) error {

}

func setRedWs(s *gameSes, ctx echo.Context) error {

}

func addReadOnlyWs(s *gameSes, ctx echo.Context) error {

}

func upgradeWS(ctx echo.Context) *websocket.Conn {
	ws, err := upgrader.Upgrade(ctx.Response(), ctx.Request(), nil)
	if err != nil {
		log.Fatal(err)
	}
	return ws
}

func ses(ctx echo.Context, st sesType) (*gameSes, error) {
	id := c.Param("id")

	for _, s := range sessions {
		var sid sesID
		switch st {
		case admin:
			sid = s.sesID
		case blue:
			sid = s.blueSesID
		case red:
			sid = s.redSesID
		}

		if sid == id {
			return s, nil
		}
	}
	return nil, c.String(http.StatusBadRequest, "no session")
}

func adminHandler(c echo.Context) error {
	return c.String(http.StatusOK, "admin")
}

func adminWsSesHandler(c echo.Context) error {
	s, err := ses(c, admin)
	if err != nil {
		return err
	}

	s.adminWs = upgradeWS(c)
	return ctx.NoContent(http.StatusSwitchingProtocols)
}

func blueWsSesHandler(c echo.Context) error {
	s, err := ses(c, blue)
	if err != nil {
		return err
	}

	s.blueWs = upgradeWS(c)
	return ctx.NoContent(http.StatusSwitchingProtocols)
}

func redWsSesHandler(c echo.Context) error {
	s, err := ses(c, red)
	if err != nil {
		return err
	}

	s.redWs = upgradeWS(c)
	return ctx.NoContent(http.StatusSwitchingProtocols)
}

// Start fires up the battlerite system. cfgDir sets where to look for config jsons, conn is the echo connection string, eg ":1234"
func Start(cfgDir string, conn string) {
	e := echo.New()
	e.GET("/admin", adminHandler)
	e.GET("/ws/admin/:id", adminSesHandler)
	e.GET("/ws/blue/:id", blueSesHandler)
	e.GET("/ws/red/:id", redSesHandler)
	e.Logger.Fatal(e.Start(conn))
}
