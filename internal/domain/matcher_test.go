package domain_test

import (
	"testing"

	"fair-dispatch/internal/config"
	"fair-dispatch/internal/domain"
)

func TestMatcherScoring(t *testing.T) {
	cfg := &config.DomainConfig{
		Weights: config.Weights{
			Fairness:  0.6,
			Proximity: 0.4,
		},
	}
	matcher := domain.NewMatcher(cfg)

	// Rider A has done 10 orders recently (fatigued/busy)
	riderA := &domain.Resource{ID: "rider-A", OrdersLastHour: 10}

	// Rider B has done 0 orders (fresh/idle)
	riderB := &domain.Resource{ID: "rider-B", OrdersLastHour: 0}

	distance := 2.0 // Assuming both are 2km away

	scoreA := matcher.Score(riderA, distance)
	scoreB := matcher.Score(riderB, distance)

	if scoreB <= scoreA {
		t.Errorf("Expected idle Rider B (score %f) to score higher than busy Rider A (score %f)", scoreB, scoreA)
	}

	t.Logf("Rider A Score: %.3f", scoreA)
	t.Logf("Rider B Score: %.3f", scoreB)
}
