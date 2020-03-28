package main

import (
	"sync"
	"testing"
	"time"
)

func createTestDraft() *draft {
	d := &draft{
		Setup: &draftSetup{
			Name:           "",
			MapName:        "",
			BlueName:       "",
			RedName:        "",
			VotingSecs:     []int{5, 5, 5, 5, 5},
			PhaseDelaySecs: 0,
		},
		IDs: &draftIDs{
			Admin:   "1a",
			Red:     "2r",
			Blue:    "3b",
			Results: "4rv",
		},
		adminWs:       nil,
		redWs:         nil,
		blueWs:        nil,
		readonlyWss:   nil,
		wsWriteMutext: sync.Mutex{},
		waitingStart:  false,
		curSnapshot: &WsMsg{
			Type:               0,
			AdminConnected:     false,
			RedConnected:       false,
			BlueConnected:      false,
			ResultsViewers:     0,
			RedReady:           false,
			BlueReady:          false,
			VoteActive:         false,
			VotePaused:         false,
			VoteTimeLeft:       0,
			VoteUnlimitedTime:  false,
			CurrentPhase:       0,
			CurrentVote:        nil,
			Phases:             nil,
			DraftDone:          false,
			DraftStarted:       false,
			DraftCreatedAt:     time.Time{},
			DraftStartedAt:     time.Time{},
			DraftEndedAt:       time.Time{},
			VotingStartedAt:    time.Time{},
			VoteTimeLeftPretty: "",
		},
	}

	return d
}

func TestDraftRules(t *testing.T) {
	d := createTestDraft()

	lenAllChamps := len(champsFlat)
	validChamps := validChampsForCurPhase(d)

	// we have no active vote, so all champs should be returned
	if len(validChamps.blue) < lenAllChamps || len(validChamps.red) < lenAllChamps {
		t.Errorf("wrong number of champs")
	}

	// first vote, should get all champs still
	d.curSnapshot.CurrentVote = &phaseVote{
		PhaseNum: 0,
	}

	validChamps = validChampsForCurPhase(d)
	if len(validChamps.blue) < lenAllChamps || len(validChamps.red) < lenAllChamps {
		t.Errorf("wrong number of champs")
	}

	// second vote, need to filter based on previous type and result
	d.curSnapshot.CurrentVote.PhaseNum = 1
	validChamps = validChampsForCurPhase(d)
	if len(validChamps.blue) < lenAllChamps || len(validChamps.red) < lenAllChamps {
		t.Errorf("wrong number of champs")
	}
}
