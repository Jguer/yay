//go:build !integration

package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeparateSourceScore_UsesRepoOrderEvenlyDistributed(t *testing.T) {
	t.Parallel()

	// Any non-1.0 score avoids the special-case 50 return.
	const sim = 0.5
	const delta = 1e-1

	t.Run("arch repos (core/extra/community/multilib)", func(t *testing.T) {
		a := &abstractResults{
			separateSources:     true,
			repoOrder:           []string{"core", "extra", "community", "multilib"},
			separateSourceCache: map[string]float64{},
		}

		assert.InDelta(t, 45.0, a.separateSourceScore("core", sim), delta)
		assert.InDelta(t, 31.6, a.separateSourceScore("extra", sim), delta)
		assert.InDelta(t, 18.3, a.separateSourceScore("community", sim), delta)
		assert.InDelta(t, 5.0, a.separateSourceScore("multilib", sim), delta)
		assert.Equal(t, 0.0, a.separateSourceScore("aur", sim))
	})

	t.Run("arch arm repos (core/extra/alarm/aur)", func(t *testing.T) {
		a := &abstractResults{
			separateSources:     true,
			repoOrder:           []string{"core", "extra", "alarm", "aur"},
			separateSourceCache: map[string]float64{},
		}

		// Note: AUR is not a sync repository; it is always lowest priority (0) regardless of repo order.
		assert.InDelta(t, 45.0, a.separateSourceScore("core", sim), delta)
		assert.InDelta(t, 31.6, a.separateSourceScore("extra", sim), delta)
		assert.InDelta(t, 18.3, a.separateSourceScore("alarm", sim), delta)
		assert.Equal(t, 0.0, a.separateSourceScore("aur", sim))
	})
}

func TestSeparateSourceScore_ExactMatchKeepsRepoOrder(t *testing.T) {
	t.Parallel()

	a := &abstractResults{
		separateSources:     true,
		repoOrder:           []string{"cachyos-extra-v3", "core", "extra"},
		separateSourceCache: map[string]float64{},
	}

	custom := a.separateSourceScore("cachyos-extra-v3", 1.0)
	extra := a.separateSourceScore("extra", 1.0)
	aurScore := a.separateSourceScore("aur", 1.0)

	assert.Greater(t, custom, extra)
	assert.Greater(t, extra, aurScore)
	assert.Equal(t, 50.0, aurScore)
	// Exact matches must still outrank any non-exact match.
	assert.Greater(t, extra+1.0, a.separateSourceScore("cachyos-extra-v3", 0.99)+0.99)
}
