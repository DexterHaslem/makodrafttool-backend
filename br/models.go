package br

import "github.com/google/uuid"

type sesID uuid.UUID

type adminSes struct {
	mapName      string
	blueName     string
	redName      string
	redSesID     sesID
	blueSesID    sesID
	resultsSesID sesID
}
