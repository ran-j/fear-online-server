package catalog_test

import (
	"testing"

	"go-service-template/internal/catalog"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	loaded, err := catalog.Load()
	assert.NoError(t, err)

	t.Run("row counts match the extracted CSVs", func(t *testing.T) {
		assert.Len(t, loaded.Items, 1295)
		assert.Len(t, loaded.Maps, 25)
		assert.Len(t, loaded.Classes, 75)
		assert.Len(t, loaded.Recipes, 18)
	})

	t.Run("GameItem resolves a known character", func(t *testing.T) {
		jack, ok := loaded.Items[12100101]
		assert.True(t, ok)
		assert.Equal(t, "MP_M_TF_01", jack.RecordName)
		assert.Equal(t, 1, jack.HighGroup) // characters are HighGroup 1
	})

	t.Run("ClassInfo carries the first rank", func(t *testing.T) {
		first := loaded.Classes[0]
		assert.Equal(t, "Recruit", first.ClassName)
		assert.Equal(t, uint64(0), first.ExpMin)
		assert.Equal(t, uint64(399), first.ExpMax)
	})

	t.Run("MapInfo parses pipe-separated room sizes", func(t *testing.T) {
		rooftop, ok := loaded.Maps[515]
		assert.True(t, ok)
		assert.Equal(t, "TAM_Rooftop", rooftop.MapName)
		assert.Equal(t, []int{8, 12, 16}, rooftop.User)
		assert.Equal(t, 2, rooftop.UserMin)
	})

	t.Run("Recipe collects its non-zero materials", func(t *testing.T) {
		recipe, ok := loaded.Recipes[92000101]
		assert.True(t, ok)
		assert.Len(t, recipe.Materials, 3)
		assert.Equal(t, uint32(21601701), recipe.ItemOutput)
	})

	t.Run("RewardItem indexes every row of a shared index", func(t *testing.T) {
		assert.NotEmpty(t, loaded.Rewards[11030000])
	})
}

func TestFunctionType(t *testing.T) {
	gameCatalog, err := catalog.Load()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	// Clan_Mark_Change points at FunctionIndex 1901, whose FunctionType is 19.
	if got := gameCatalog.FunctionType(44510502); got != 19 {
		t.Fatalf("Clan_Mark_Change function type = %d, want 19", got)
	}
	// The free variant shares the function.
	if got := gameCatalog.FunctionType(44510501); got != 19 {
		t.Fatalf("free Clan_Mark_Change function type = %d, want 19", got)
	}
	// A plain weapon has no function at all.
	if got := gameCatalog.FunctionType(21102201); got != 0 {
		t.Fatalf("M16 function type = %d, want 0", got)
	}
}
