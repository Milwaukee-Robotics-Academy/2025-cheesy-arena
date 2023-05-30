// Copyright 2018 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Contains configuration of the publish-subscribe notifiers that allow the arena to push updates to websocket clients.

package field

import (
	"github.com/Team254/cheesy-arena/bracket"
	"github.com/Team254/cheesy-arena/game"
	"github.com/Team254/cheesy-arena/model"
	"github.com/Team254/cheesy-arena/network"
	"github.com/Team254/cheesy-arena/websocket"
	"strconv"
)

type ArenaNotifiers struct {
	AllianceSelectionNotifier          *websocket.Notifier
	AllianceStationDisplayModeNotifier *websocket.Notifier
	ArenaStatusNotifier                *websocket.Notifier
	AudienceDisplayModeNotifier        *websocket.Notifier
	DisplayConfigurationNotifier       *websocket.Notifier
	EventStatusNotifier                *websocket.Notifier
	LowerThirdNotifier                 *websocket.Notifier
	MatchLoadNotifier                  *websocket.Notifier
	MatchTimeNotifier                  *websocket.Notifier
	MatchTimingNotifier                *websocket.Notifier
	PlaySoundNotifier                  *websocket.Notifier
	RealtimeScoreNotifier              *websocket.Notifier
	ReloadDisplaysNotifier             *websocket.Notifier
	ScorePostedNotifier                *websocket.Notifier
	ScoringStatusNotifier              *websocket.Notifier
}

type MatchTimeMessage struct {
	MatchState
	MatchTimeSec int
}

type audienceAllianceScoreFields struct {
	Score        *game.Score
	ScoreSummary *game.ScoreSummary
}

// Instantiates notifiers and configures their message producing methods.
func (arena *Arena) configureNotifiers() {
	arena.AllianceSelectionNotifier = websocket.NewNotifier("allianceSelection", arena.generateAllianceSelectionMessage)
	arena.AllianceStationDisplayModeNotifier = websocket.NewNotifier("allianceStationDisplayMode",
		arena.generateAllianceStationDisplayModeMessage)
	arena.ArenaStatusNotifier = websocket.NewNotifier("arenaStatus", arena.generateArenaStatusMessage)
	arena.AudienceDisplayModeNotifier = websocket.NewNotifier("audienceDisplayMode",
		arena.generateAudienceDisplayModeMessage)
	arena.DisplayConfigurationNotifier = websocket.NewNotifier("displayConfiguration",
		arena.generateDisplayConfigurationMessage)
	arena.EventStatusNotifier = websocket.NewNotifier("eventStatus", arena.generateEventStatusMessage)
	arena.LowerThirdNotifier = websocket.NewNotifier("lowerThird", arena.generateLowerThirdMessage)
	arena.MatchLoadNotifier = websocket.NewNotifier("matchLoad", arena.GenerateMatchLoadMessage)
	arena.MatchTimeNotifier = websocket.NewNotifier("matchTime", arena.generateMatchTimeMessage)
	arena.MatchTimingNotifier = websocket.NewNotifier("matchTiming", arena.generateMatchTimingMessage)
	arena.PlaySoundNotifier = websocket.NewNotifier("playSound", nil)
	arena.RealtimeScoreNotifier = websocket.NewNotifier("realtimeScore", arena.generateRealtimeScoreMessage)
	arena.ReloadDisplaysNotifier = websocket.NewNotifier("reload", nil)
	arena.ScorePostedNotifier = websocket.NewNotifier("scorePosted", arena.GenerateScorePostedMessage)
	arena.ScoringStatusNotifier = websocket.NewNotifier("scoringStatus", arena.generateScoringStatusMessage)
}

func (arena *Arena) generateAllianceSelectionMessage() any {
	return &arena.AllianceSelectionAlliances
}

func (arena *Arena) generateAllianceStationDisplayModeMessage() any {
	return arena.AllianceStationDisplayMode
}

func (arena *Arena) generateArenaStatusMessage() any {
	// Convert AP team wifi network status array to a map by station for ease of client use.
	teamWifiStatuses := make(map[string]network.TeamWifiStatus)
	for i, station := range []string{"R1", "R2", "R3", "B1", "B2", "B3"} {
		if arena.EventSettings.Ap2TeamChannel == 0 || i < 3 {
			teamWifiStatuses[station] = arena.accessPoint.TeamWifiStatuses[i]
		} else {
			teamWifiStatuses[station] = arena.accessPoint2.TeamWifiStatuses[i]
		}
	}

	return &struct {
		MatchId          int
		AllianceStations map[string]*AllianceStation
		TeamWifiStatuses map[string]network.TeamWifiStatus
		MatchState
		CanStartMatch         bool
		PlcIsHealthy          bool
		FieldEstop            bool
		PlcArmorBlockStatuses map[string]bool
	}{arena.CurrentMatch.Id, arena.AllianceStations, teamWifiStatuses, arena.MatchState,
		arena.checkCanStartMatch() == nil, arena.Plc.IsHealthy(), arena.Plc.GetFieldEstop(),
		arena.Plc.GetArmorBlockStatuses()}
}

func (arena *Arena) generateAudienceDisplayModeMessage() any {
	return arena.AudienceDisplayMode
}

func (arena *Arena) generateDisplayConfigurationMessage() any {
	// Notify() for this notifier must always called from a method that has a lock on the display mutex.
	// Make a copy of the map to avoid potential data races; otherwise the same map would get iterated through as it is
	// serialized to JSON, outside the mutex lock.
	displaysCopy := make(map[string]Display)
	for displayId, display := range arena.Displays {
		displaysCopy[displayId] = *display
	}
	return displaysCopy
}

func (arena *Arena) generateEventStatusMessage() any {
	return arena.EventStatus
}

func (arena *Arena) generateLowerThirdMessage() any {
	return &struct {
		LowerThird     *model.LowerThird
		ShowLowerThird bool
	}{arena.LowerThird, arena.ShowLowerThird}
}

func (arena *Arena) GenerateMatchLoadMessage() any {
	teams := make(map[string]*model.Team)
	for station, allianceStation := range arena.AllianceStations {
		teams[station] = allianceStation.Team
	}

	rankings := make(map[string]*game.Ranking)
	for _, allianceStation := range arena.AllianceStations {
		if allianceStation.Team != nil {
			rankings[strconv.Itoa(allianceStation.Team.Id)], _ =
				arena.Database.GetRankingForTeam(allianceStation.Team.Id)
		}
	}

	var matchup *bracket.Matchup
	redOffFieldTeams := []*model.Team{}
	blueOffFieldTeams := []*model.Team{}
	if arena.CurrentMatch.Type == model.Playoff {
		matchup, _ = arena.PlayoffBracket.GetMatchup(arena.CurrentMatch.PlayoffRound, arena.CurrentMatch.PlayoffGroup)
		redOffFieldTeamIds, blueOffFieldTeamIds, _ := arena.Database.GetOffFieldTeamIds(arena.CurrentMatch)
		for _, teamId := range redOffFieldTeamIds {
			team, _ := arena.Database.GetTeamById(teamId)
			redOffFieldTeams = append(redOffFieldTeams, team)
		}
		for _, teamId := range blueOffFieldTeamIds {
			team, _ := arena.Database.GetTeamById(teamId)
			blueOffFieldTeams = append(blueOffFieldTeams, team)
		}
	}

	return &struct {
		Match             *model.Match
		Teams             map[string]*model.Team
		Rankings          map[string]*game.Ranking
		Matchup           *bracket.Matchup
		RedOffFieldTeams  []*model.Team
		BlueOffFieldTeams []*model.Team
	}{
		arena.CurrentMatch,
		teams,
		rankings,
		matchup,
		redOffFieldTeams,
		blueOffFieldTeams,
	}
}

func (arena *Arena) generateMatchTimeMessage() any {
	return MatchTimeMessage{arena.MatchState, int(arena.MatchTimeSec())}
}

func (arena *Arena) generateMatchTimingMessage() any {
	return &game.MatchTiming
}

func (arena *Arena) generateRealtimeScoreMessage() any {
	fields := struct {
		Red       *audienceAllianceScoreFields
		Blue      *audienceAllianceScoreFields
		RedCards  map[string]string
		BlueCards map[string]string
		MatchState
	}{
		getAudienceAllianceScoreFields(arena.RedRealtimeScore, arena.RedScoreSummary()),
		getAudienceAllianceScoreFields(arena.BlueRealtimeScore, arena.BlueScoreSummary()),
		arena.RedRealtimeScore.Cards,
		arena.BlueRealtimeScore.Cards,
		arena.MatchState,
	}
	return &fields
}

func (arena *Arena) GenerateScorePostedMessage() any {
	redScoreSummary := arena.SavedMatchResult.RedScoreSummary()
	blueScoreSummary := arena.SavedMatchResult.BlueScoreSummary()
	redRankingPoints := redScoreSummary.BonusRankingPoints
	blueRankingPoints := blueScoreSummary.BonusRankingPoints
	switch arena.SavedMatch.Status {
	case game.RedWonMatch:
		redRankingPoints += 2
	case game.BlueWonMatch:
		blueRankingPoints += 2
	case game.TieMatch:
		redRankingPoints++
		blueRankingPoints++
	}

	// For playoff matches, summarize the state of the series.
	var seriesStatus, seriesLeader string
	var matchup *bracket.Matchup
	redOffFieldTeamIds := []int{}
	blueOffFieldTeamIds := []int{}
	if arena.SavedMatch.Type == model.Playoff {
		matchup, _ = arena.PlayoffBracket.GetMatchup(arena.SavedMatch.PlayoffRound, arena.SavedMatch.PlayoffGroup)
		seriesLeader, seriesStatus = matchup.StatusText()
		redOffFieldTeamIds, blueOffFieldTeamIds, _ = arena.Database.GetOffFieldTeamIds(arena.SavedMatch)
	}

	redRankings := map[int]*game.Ranking{
		arena.SavedMatch.Red1: nil, arena.SavedMatch.Red2: nil, arena.SavedMatch.Red3: nil,
	}
	blueRankings := map[int]*game.Ranking{
		arena.SavedMatch.Blue1: nil, arena.SavedMatch.Blue2: nil, arena.SavedMatch.Blue3: nil,
	}
	for index, ranking := range arena.SavedRankings {
		if _, ok := redRankings[ranking.TeamId]; ok {
			redRankings[ranking.TeamId] = &arena.SavedRankings[index]
		}
		if _, ok := blueRankings[ranking.TeamId]; ok {
			blueRankings[ranking.TeamId] = &arena.SavedRankings[index]
		}
	}

	return &struct {
		Match               *model.Match
		RedScoreSummary     *game.ScoreSummary
		BlueScoreSummary    *game.ScoreSummary
		RedRankingPoints    int
		BlueRankingPoints   int
		RedFouls            []game.Foul
		BlueFouls           []game.Foul
		RulesViolated       map[int]*game.Rule
		RedCards            map[string]string
		BlueCards           map[string]string
		RedRankings         map[int]*game.Ranking
		BlueRankings        map[int]*game.Ranking
		RedOffFieldTeamIds  []int
		BlueOffFieldTeamIds []int
		SeriesStatus        string
		SeriesLeader        string
	}{
		arena.SavedMatch,
		redScoreSummary,
		blueScoreSummary,
		redRankingPoints,
		blueRankingPoints,
		arena.SavedMatchResult.RedScore.Fouls,
		arena.SavedMatchResult.BlueScore.Fouls,
		getRulesViolated(arena.SavedMatchResult.RedScore.Fouls, arena.SavedMatchResult.BlueScore.Fouls),
		arena.SavedMatchResult.RedCards,
		arena.SavedMatchResult.BlueCards,
		redRankings,
		blueRankings,
		redOffFieldTeamIds,
		blueOffFieldTeamIds,
		seriesStatus,
		seriesLeader,
	}
}

func (arena *Arena) generateScoringStatusMessage() any {
	return &struct {
		RefereeScoreReady         bool
		RedScoreReady             bool
		BlueScoreReady            bool
		NumRedScoringPanels       int
		NumRedScoringPanelsReady  int
		NumBlueScoringPanels      int
		NumBlueScoringPanelsReady int
	}{arena.RedRealtimeScore.FoulsCommitted && arena.BlueRealtimeScore.FoulsCommitted,
		arena.alliancePostMatchScoreReady("red"), arena.alliancePostMatchScoreReady("blue"),
		arena.ScoringPanelRegistry.GetNumPanels("red"), arena.ScoringPanelRegistry.GetNumScoreCommitted("red"),
		arena.ScoringPanelRegistry.GetNumPanels("blue"), arena.ScoringPanelRegistry.GetNumScoreCommitted("blue")}
}

// Constructs the data object for one alliance sent to the audience display for the realtime scoring overlay.
func getAudienceAllianceScoreFields(allianceScore *RealtimeScore,
	allianceScoreSummary *game.ScoreSummary) *audienceAllianceScoreFields {
	fields := new(audienceAllianceScoreFields)
	fields.Score = &allianceScore.CurrentScore
	fields.ScoreSummary = allianceScoreSummary
	return fields
}

// Produce a map of rules that were violated by either alliance so that they are available to the announcer.
func getRulesViolated(redFouls, blueFouls []game.Foul) map[int]*game.Rule {
	rules := make(map[int]*game.Rule)
	for _, foul := range redFouls {
		rules[foul.RuleId] = game.GetRuleById(foul.RuleId)
	}
	for _, foul := range blueFouls {
		rules[foul.RuleId] = game.GetRuleById(foul.RuleId)
	}
	return rules
}
