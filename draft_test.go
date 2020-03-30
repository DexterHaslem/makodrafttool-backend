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

	readGameJsons("./cfg")

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
	d.curSnapshot.CurrentPhase = 1
	d.curSnapshot.CurrentVote.PhaseType = phaseTypeBan
	validChamps = validChampsForCurPhase(d)
	if len(validChamps.blue) < lenAllChamps || len(validChamps.red) < lenAllChamps {
		t.Errorf("wrong number of champs")
	}

	// add first phase (ban) and add picks

	p1banRed := "bakko"
	p1banBlue := "croak"
	d.curSnapshot.Phases = []*phaseVote{
		&phaseVote{
			PhaseType:     phaseTypeBan,
			PhaseNum:      0,
			VoteRedValue:  p1banRed,
			VoteBlueValue: p1banBlue,
		},
	}

	/* all champs enabled during ban for now
	validChamps = validChampsForCurPhase(d)

	for _, blueChamp := range validChamps.blue {
		if blueChamp == p1banRed {
			t.Errorf("blue able to pick red's ban")
		}
	}

	for _, redChamp := range validChamps.red {
		if redChamp == p1banBlue {
			t.Errorf("red able to pick blue's ban")
		}
	}
	*/
	// check picks after some bans. this isnt valid order but it tests bans
	d.curSnapshot.CurrentPhase = 3 // zero indexed
	d.curSnapshot.CurrentVote.PhaseType = phaseTypePick

	d.curSnapshot.Phases = []*phaseVote{
		&phaseVote{
			PhaseType:     phaseTypeBan,
			PhaseNum:      0,
			VoteRedValue:  "bakko",
			VoteBlueValue: "zander",
		},

		&phaseVote{
			PhaseType:     phaseTypePick,
			PhaseNum:      1,
			VoteRedValue:  "croak",
			VoteBlueValue: "freya",
		},

		&phaseVote{
			PhaseType:     phaseTypePick,
			PhaseNum:      2,
			VoteRedValue:  "jamila",
			VoteBlueValue: "raigon",
		},
	}

	validChamps = validChampsForCurPhase(d)

	// first ensure red pick options are correct
	redGotRaigon := false
	redGotFreya := false
	for _, rc := range validChamps.red {
		if rc == "zander" {
			t.Fatalf("red was able to pick blue ban #1")
		}

		if rc == "croak" {
			t.Fatalf("red was able to pick reds previous pick 1")
		}

		if rc == "jamila" {
			t.Fatalf("red was able to pick reds previous pick 2")
		}

		if rc == "bakko" {
			t.Fatalf("red was able to pick a ban already banned by red")
		}

		if rc == "raigon" {
			redGotRaigon = true
		}
		if rc == "freya" {
			redGotFreya = true
		}
	}

	if !redGotFreya || !redGotRaigon {
		t.Fatalf("red was unable to pick a valid pick because blue picked it")
	}

	// now check blue options are ok
	// for _, bc := range validChamps.blue {
	// }
}
