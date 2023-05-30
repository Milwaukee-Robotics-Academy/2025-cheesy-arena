// Copyright 2022 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Helper methods for use in tests in this package and others.

package bracket

import (
	"github.com/Team254/cheesy-arena/game"
	"github.com/Team254/cheesy-arena/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func setupTestDb(t *testing.T) *model.Database {
	return model.SetupTestDb(t, "bracket")
}

func assertMatch(
	t *testing.T,
	match model.Match,
	// TODO(pat): Also assert type order here
	longName, shortName, nameDetail string,
	redAlliance int,
	blueAlliance int,
) {
	assert.Equal(t, longName, match.LongName)
	assert.Equal(t, shortName, match.ShortName)
	assert.Equal(t, nameDetail, match.NameDetail)
	assert.Equal(t, redAlliance, match.PlayoffRedAlliance)
	assert.Equal(t, blueAlliance, match.PlayoffBlueAlliance)
	assert.Equal(t, 100*redAlliance+2, match.Red1)
	assert.Equal(t, 100*redAlliance+1, match.Red2)
	assert.Equal(t, 100*redAlliance+3, match.Red3)
	assert.Equal(t, 100*blueAlliance+2, match.Blue1)
	assert.Equal(t, 100*blueAlliance+1, match.Blue2)
	assert.Equal(t, 100*blueAlliance+3, match.Blue3)
}

func scoreMatch(database *model.Database, shortName string, winner game.MatchStatus) {
	match, _ := database.GetMatchByName(model.Playoff, shortName)
	match.Status = winner
	database.UpdateMatch(match)
	database.UpdateAllianceFromMatch(match.PlayoffRedAlliance, [3]int{match.Red1, match.Red2, match.Red3})
	database.UpdateAllianceFromMatch(match.PlayoffBlueAlliance, [3]int{match.Blue1, match.Blue2, match.Blue3})
}
