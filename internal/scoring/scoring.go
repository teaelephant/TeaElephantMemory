package scoring

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Candidate represents a tea candidate for Tea of the Day selection.
// It intentionally contains only the fields required for scoring to avoid
// importing API model packages and creating cycles.
type Candidate struct {
	ID         uuid.UUID
	Name       string
	Expiration time.Time // earliest expiration among user records for this tea; zero if unknown
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
	best := uuid.Nil
	bestScore := -1 << 30

	var bestExp time.Time

	for _, c := range candidates {
		score := 0

		// AI-provided context score (weather + day-of-week), clamp to [0,15]
		if v, ok := aiScores[c.ID]; ok {
			if v < 0 {
				v = 0
			} else if v > 15 {
				v = 15
			}

			score += v
		}

		// Recent consumption penalty
		if last, ok := lastByTea[c.ID]; ok {
			diff := now.Sub(last)
			if diff <= 24*time.Hour {
				score -= 5
			} else if diff <= 48*time.Hour {
				score -= 3
			}
		}

		// Expiration bonus
		if !c.Expiration.IsZero() {
			delta := c.Expiration.Sub(now)
			if delta <= 7*24*time.Hour {
				score += 5
			} else if delta <= 30*24*time.Hour {
				score += 2
			}
		}

		// Compare with current best. Tie-breakers: earliest expiration, then lexical by name.
		switch {
		case score > bestScore:
			best = c.ID
			bestScore = score
			bestExp = c.Expiration
		case score == bestScore:
			if !c.Expiration.IsZero() {
				if bestExp.IsZero() || c.Expiration.Before(bestExp) {
					best = c.ID
					bestExp = c.Expiration
				}
			} else if c.Expiration.Equal(bestExp) {
				// stable tie-breaker by name
				if strings.Compare(c.Name, candidateName(candidates, best)) < 0 {
					best = c.ID
				}
			}
		}
	}

	return best, bestScore
}

func candidateName(cands []Candidate, id uuid.UUID) string {
	for _, c := range cands {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}
