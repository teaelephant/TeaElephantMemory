package scoring

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Scoring constants to avoid magic numbers and clarify intent.
const (
	// AI context score bounds (inclusive)
	aiScoreMin = 0
	aiScoreMax = 15

	// Recent consumption penalty thresholds
	recent24h = 24 * time.Hour
	recent48h = 48 * time.Hour

	// Penalties applied when tea was consumed recently
	penaltyWithin24h = 5
	penaltyWithin48h = 3

	// Expiration bonus thresholds
	expSoonThreshold     = 7 * 24 * time.Hour
	expUpcomingThreshold = 30 * 24 * time.Hour

	// Bonuses for approaching expiration
	bonusExpSoon     = 3
	bonusExpUpcoming = 1

	// Initial very low score to ensure first candidate wins the first comparison
	initialBestScore = -1 << 30
)

// Candidate represents a tea candidate for Tea of the Day selection.
// It intentionally contains only the fields required for scoring to avoid
// importing API model packages and creating cycles.
type Candidate struct {
	ID         uuid.UUID
	Name       string
	Expiration time.Time // earliest expiration among user records for this tea; zero if unknown
}

func clampedAIScore(aiScores map[uuid.UUID]int, id uuid.UUID) int {
	v, ok := aiScores[id]
	if !ok {
		return 0
	}

	if v < aiScoreMin {
		return aiScoreMin
	}

	if v > aiScoreMax {
		return aiScoreMax
	}

	return v
}

func recentPenalty(lastByTea map[uuid.UUID]time.Time, id uuid.UUID, now time.Time) int {
	last, ok := lastByTea[id]
	if !ok {
		return 0
	}

	diff := now.Sub(last)
	if diff <= recent24h {
		return -penaltyWithin24h
	}

	if diff <= recent48h {
		return -penaltyWithin48h
	}

	return 0
}

func expirationBonus(exp time.Time, now time.Time) int {
	if exp.IsZero() {
		return 0
	}

	delta := exp.Sub(now)
	if delta <= expSoonThreshold {
		return bonusExpSoon
	}

	if delta <= expUpcomingThreshold {
		return bonusExpUpcoming
	}

	return 0
}

func betterCandidate(curr Candidate, currScore int, best Candidate, bestScore int, candidates []Candidate) bool {
	if currScore > bestScore {
		return true
	}

	if currScore < bestScore {
		return false
	}
	// equal scores: tie-breakers
	if !curr.Expiration.IsZero() {
		if best.Expiration.IsZero() || curr.Expiration.Before(best.Expiration) {
			return true
		}

		if curr.Expiration.After(best.Expiration) {
			return false
		}
	}
	// same expiration (or both zero) -> lexical by name
	return strings.Compare(curr.Name, candidateName(candidates, best.ID)) < 0
}

// SelectBest selects the best tea according to the scoring rules.
// Inputs:
// - aiScores: context-aware scores (0..15) provided by AI per tea ID (weather + day-of-week)
// - candidates: list of candidates with expiration/name
// - lastByTea: most recent consumption time per tea ID
// - now: current time
// Returns the ID of the best tea and its total score.
func SelectBest(
	aiScores map[uuid.UUID]int,
	candidates []Candidate,
	lastByTea map[uuid.UUID]time.Time,
	now time.Time,
) (uuid.UUID, int) {
	best := Candidate{}
	bestScore := initialBestScore

	for _, c := range candidates {
		score := 0
		score += clampedAIScore(aiScores, c.ID)
		score += recentPenalty(lastByTea, c.ID, now)
		score += expirationBonus(c.Expiration, now)

		if best.ID == uuid.Nil || betterCandidate(c, score, best, bestScore, candidates) {
			best = c
			bestScore = score
		}
	}

	return best.ID, bestScore
}

func candidateName(cands []Candidate, id uuid.UUID) string {
	for _, c := range cands {
		if c.ID == id {
			return c.Name
		}
	}

	return ""
}

// LogFunc is a lightweight logging callback used by SelectBestWithLogging to avoid
// bringing a logging dependency into the scoring package. The provided fields map
// is expected to be safe for read-only use by the callee.
type LogFunc func(fields map[string]interface{}, msg string)

func formatTimeRFC3339OrDash(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}

// SelectBestWithLogging selects the best tea and emits detailed logs via the provided
// logf callback. If logf is nil, no logs are emitted. It preserves the same scoring
// rules as SelectBest and returns the chosen tea ID and its total score.
func SelectBestWithLogging(
	aiScores map[uuid.UUID]int,
	candidates []Candidate,
	lastByTea map[uuid.UUID]time.Time,
	now time.Time,
	logf LogFunc,
) (uuid.UUID, int) {
	best := Candidate{}
	bestScore := initialBestScore

	for _, c := range candidates {
		aiRaw := aiScores[c.ID]
		aiClamped := clampedAIScore(aiScores, c.ID)
		recent := recentPenalty(lastByTea, c.ID, now)
		expBonus := expirationBonus(c.Expiration, now)
		total := aiClamped + recent + expBonus

		if logf != nil {
			fields := map[string]interface{}{
				"name":            c.Name,
				"id":              c.ID.String(),
				"aiRaw":           aiRaw,
				"aiClamped":       aiClamped,
				"recentPenalty":   recent,
				"expirationBonus": expBonus,
				"total":           total,
				"lastConsumption": formatTimeRFC3339OrDash(lastByTea[c.ID]),
				"expiration":      formatTimeRFC3339OrDash(c.Expiration),
			}
			logf(fields, "tea_of_day candidate")
		}

		if best.ID == uuid.Nil || betterCandidate(c, total, best, bestScore, candidates) {
			best = c
			bestScore = total
		}
	}

	if logf != nil && best.ID != uuid.Nil {
		aiRaw := aiScores[best.ID]
		aiClamped := clampedAIScore(aiScores, best.ID)
		recent := recentPenalty(lastByTea, best.ID, now)
		expBonus := expirationBonus(best.Expiration, now)
		fields := map[string]interface{}{
			"name":            best.Name,
			"id":              best.ID.String(),
			"aiClamped":       aiClamped,
			"recentPenalty":   recent,
			"expirationBonus": expBonus,
			"total":           aiClamped + recent + expBonus,
			"lastConsumption": formatTimeRFC3339OrDash(lastByTea[best.ID]),
			"expiration":      formatTimeRFC3339OrDash(best.Expiration),
		}
		// Keep aiRaw in selection log as well for completeness
		fields["aiRaw"] = aiRaw
		logf(fields, "tea_of_day selected")
	}

	return best.ID, bestScore
}
