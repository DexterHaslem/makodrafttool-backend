package br

import (
	"net/http"

	"github.com/labstack/echo"
)

func adminHandler(c echo.Context) error {
	return c.String(http.StatusOK, "admin")
}

func adminSesHandler(c echo.Context) error {
	return c.String(http.StatusOK, "admin")
}

func blueSesHandler(c echo.Context) error {
	return c.String(http.StatusOK, "blue")
}

func redSesHandler(c echo.Context) error {
	return c.String(http.StatusOK, "red")
}
