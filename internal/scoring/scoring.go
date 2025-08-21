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
