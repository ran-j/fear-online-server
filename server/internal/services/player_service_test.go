package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-service-template/internal/catalog"
	"go-service-template/internal/models"
	"go-service-template/internal/repositories"
	"go-service-template/internal/services"

	"github.com/stretchr/testify/assert"
)

func newService(t *testing.T, players repositories.PlayerRepository) *services.PlayerService {
	t.Helper()
	gameCatalog, err := catalog.Load()
	assert.NoError(t, err)
	return services.NewPlayerService(players, gameCatalog, 50000, 50000)
}

func TestPlayerServiceLoginBySteam(t *testing.T) {
	t.Run("creates a new player with placeholder name and starting values", func(t *testing.T) {
		service := newService(t, repositories.NewMemoryPlayerRepository())

		player, err := service.LoginBySteam(context.Background(), "55555555555555555")

		assert.NoError(t, err)
		assert.Equal(t, "55555555555555555", player.SteamID)
		assert.Equal(t, "Player55555", player.Name)
		assert.Equal(t, uint32(1), player.Level)
		assert.Equal(t, uint64(50000), player.Point)
		assert.Equal(t, uint64(50000), player.Cash)
		assert.False(t, player.CreatedAt.IsZero())
	})

	t.Run("seeds the starter kit into inventory and loadout", func(t *testing.T) {
		service := newService(t, repositories.NewMemoryPlayerRepository())

		player, err := service.LoginBySteam(context.Background(), "55555555555555555")
		assert.NoError(t, err)

		assert.Len(t, player.Inventory, 3)
		assert.Len(t, player.Loadout, 3)

		slots := map[uint8]bool{}
		for _, equipped := range player.Loadout {
			slots[equipped.Slot] = true
		}
		assert.True(t, slots[models.SlotCharacterATC], "ATC character equipped")
		assert.True(t, slots[models.SlotCharacterTF], "TF character equipped")
		assert.True(t, slots[models.SlotWeaponFirst], "primary weapon equipped")
	})

	t.Run("keeps the same account on repeated logins", func(t *testing.T) {
		service := newService(t, repositories.NewMemoryPlayerRepository())

		first, err := service.LoginBySteam(context.Background(), "55555555555555555")
		assert.NoError(t, err)
		second, err := service.LoginBySteam(context.Background(), "55555555555555555")
		assert.NoError(t, err)

		assert.Equal(t, first.Name, second.Name)
		assert.Equal(t, first.CreatedAt, second.CreatedAt)
	})

	t.Run("propagates repository errors", func(t *testing.T) {
		service := newService(t, failingPlayerRepository{})

		_, err := service.LoginBySteam(context.Background(), "76561198114269084")

		assert.Error(t, err)
	})
}

func TestPlayerServiceEquip(t *testing.T) {
	t.Run("moves an owned item into its slot, replacing the previous one", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		player := models.Player{
			SteamID: "steam-1",
			Inventory: []models.InventoryItem{
				{Serial: 10, ItemIndex: 21102201, ItemType: models.SlotWeaponFirst},
				{Serial: 11, ItemIndex: 21102202, ItemType: models.SlotWeaponFirst},
			},
			Loadout: models.Loadout{{Slot: models.SlotWeaponFirst, Serial: 10}},
		}
		assert.NoError(t, repo.Create(context.Background(), player))

		updated, err := service.Equip(context.Background(), "steam-1", 11)
		assert.NoError(t, err)
		assert.Len(t, updated.Loadout, 1)
		assert.Equal(t, models.EquippedItem{Slot: models.SlotWeaponFirst, Serial: 11}, updated.Loadout[0])

		stored, _, _ := repo.Find(context.Background(), "steam-1")
		assert.Equal(t, uint16(11), stored.Loadout[0].Serial, "equip is persisted")
	})

	t.Run("rejects a serial the player does not own", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), models.Player{SteamID: "steam-2"}))

		_, err := service.Equip(context.Background(), "steam-2", 999)
		assert.ErrorIs(t, err, services.ErrItemNotOwned)
	})

	t.Run("fails when the account is missing", func(t *testing.T) {
		service := newService(t, repositories.NewMemoryPlayerRepository())

		_, err := service.Equip(context.Background(), "ghost", 1)
		assert.ErrorIs(t, err, services.ErrPlayerNotFound)
	})
}

func TestPlayerServiceRental(t *testing.T) {
	rental := catalog.GameItem{
		ItemIndex: 63400102, HighGroup: 6, MiddleGroup: 3,
		IsSell: true, IsShopShow: true, PointPrice: 420, IsMerge: true,
		UseTime: 86400, // 1 day
	}
	build := func() (*services.PlayerService, *repositories.MemoryPlayerRepository) {
		repo := repositories.NewMemoryPlayerRepository()
		gameCatalog := &catalog.Catalog{Items: map[uint32]catalog.GameItem{rental.ItemIndex: rental}}
		return services.NewPlayerService(repo, gameCatalog, 5000, 5000), repo
	}

	t.Run("buying a rental sets its expiry from UseTime", func(t *testing.T) {
		service, _ := build()
		_, err := service.LoginBySteam(context.Background(), "renter")
		assert.NoError(t, err)

		result, err := service.Buy(context.Background(), "renter", rental.ItemIndex)
		assert.NoError(t, err)
		assert.True(t, result.Item.IsRental(), "rental has an expiry")

		left := result.Item.WireValue(time.Now().UTC())
		assert.InDelta(t, 86400, left, 5, "roughly one day of seconds left")
	})

	t.Run("re-buying extends the rental instead of stacking", func(t *testing.T) {
		service, repo := build()
		_, err := service.LoginBySteam(context.Background(), "renter")
		assert.NoError(t, err)

		first, err := service.Buy(context.Background(), "renter", rental.ItemIndex)
		assert.NoError(t, err)
		second, err := service.Buy(context.Background(), "renter", rental.ItemIndex)
		assert.NoError(t, err)

		assert.False(t, second.IsNew)
		assert.InDelta(t, 2*86400, second.Item.WireValue(time.Now().UTC()), 5, "two days after re-buying")
		assert.True(t, second.Item.ExpiresAt.After(first.Item.ExpiresAt))

		stored, _, _ := repo.Find(context.Background(), "renter")
		assert.Len(t, stored.Inventory, 1, "extended, not a second row")
	})

	t.Run("expired rentals are dropped on login, with their attachments", func(t *testing.T) {
		service, repo := build()
		past := time.Now().UTC().Add(-time.Hour)
		assert.NoError(t, repo.Create(context.Background(), models.Player{
			SteamID: "expired",
			Inventory: []models.InventoryItem{
				{Serial: 1, ItemIndex: 11100101, ItemType: models.SlotCharacterATC},
				{Serial: 2, ItemIndex: 63400102, ItemType: 0x20, ExpiresAt: past},
			},
			Attachments: models.Attachments{{ParentSerial: 1, ChildSerial: 2, Slot: 0x20}},
			Loadout:     models.Loadout{{Slot: 0x20, Serial: 2}},
		}))

		player, err := service.LoginBySteam(context.Background(), "expired")
		assert.NoError(t, err)
		assert.Len(t, player.Inventory, 1, "expired rental gone")
		assert.Empty(t, player.Attachments, "its attachment gone too")
		assert.Empty(t, player.Loadout, "and its loadout entry")

		stored, _, _ := repo.Find(context.Background(), "expired")
		assert.Len(t, stored.Inventory, 1, "purge persisted")
	})

	t.Run("an expired item cannot be equipped", func(t *testing.T) {
		service, repo := build()
		assert.NoError(t, repo.Create(context.Background(), models.Player{
			SteamID:   "stale",
			Inventory: []models.InventoryItem{{Serial: 5, ItemIndex: 63400102, ItemType: 0x20, ExpiresAt: time.Now().UTC().Add(-time.Minute)}},
		}))

		_, err := service.Equip(context.Background(), "stale", 5)
		assert.ErrorIs(t, err, services.ErrItemExpired)
	})
}

func TestPlayerServiceAttach(t *testing.T) {
	newPlayer := func() models.Player {
		return models.Player{
			SteamID: "wearer",
			Inventory: []models.InventoryItem{
				{Serial: 1, ItemIndex: 11100101, ItemType: models.SlotCharacterATC},
				{Serial: 17, ItemIndex: 32400202, ItemType: 0x20}, // glasses
				{Serial: 18, ItemIndex: 32400303, ItemType: 0x20}, // other glasses
			},
		}
	}

	t.Run("mounts gear on the character and persists", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), newPlayer()))

		updated, child, err := service.Attach(context.Background(), "wearer", 1, 17)
		assert.NoError(t, err)
		assert.Equal(t, uint32(32400202), child.ItemIndex)
		assert.Equal(t, models.Attachments{{ParentSerial: 1, ChildSerial: 17, Slot: 0x20}}, updated.Attachments)

		stored, _, _ := repo.Find(context.Background(), "wearer")
		assert.Len(t, stored.Attachments, 1, "attachment persisted")
	})

	t.Run("replaces whatever occupied that parent's slot", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), newPlayer()))

		_, _, err := service.Attach(context.Background(), "wearer", 1, 17)
		assert.NoError(t, err)
		updated, _, err := service.Attach(context.Background(), "wearer", 1, 18)
		assert.NoError(t, err)

		assert.Len(t, updated.Attachments, 1, "same slot, not a second row")
		assert.Equal(t, uint16(18), updated.Attachments[0].ChildSerial)
	})

	t.Run("rejects a child the player does not own", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), newPlayer()))

		_, _, err := service.Attach(context.Background(), "wearer", 1, 999)
		assert.ErrorIs(t, err, services.ErrItemNotOwned)
	})
}

func TestPlayerServiceBuy(t *testing.T) {
	// build a service whose catalog holds exactly one item, so prices are known.
	build := func(item catalog.GameItem, point, cash uint64) (*services.PlayerService, *repositories.MemoryPlayerRepository) {
		repo := repositories.NewMemoryPlayerRepository()
		gameCatalog := &catalog.Catalog{Items: map[uint32]catalog.GameItem{item.ItemIndex: item}}
		return services.NewPlayerService(repo, gameCatalog, point, cash), repo
	}

	t.Run("charges point, adds the item, and persists", func(t *testing.T) {
		service, repo := build(catalog.GameItem{
			ItemIndex: 99999, HighGroup: 2, MiddleGroup: 1,
			IsSell: true, IsShopShow: true, PointPrice: 1000,
		}, 5000, 5000)
		_, err := service.LoginBySteam(context.Background(), "buyer")
		assert.NoError(t, err)

		result, err := service.Buy(context.Background(), "buyer", 99999)
		assert.NoError(t, err)
		assert.True(t, result.IsNew)
		assert.Equal(t, models.WalletPoint, result.Wallet)
		assert.Equal(t, uint64(1000), result.Price)
		assert.Equal(t, uint64(4000), result.Player.Point)

		stored, _, _ := repo.Find(context.Background(), "buyer")
		assert.Len(t, stored.Inventory, 1)
		assert.Equal(t, uint64(4000), stored.Point, "debit persisted")
		assert.Equal(t, uint64(5000), stored.Cash, "cash untouched")
	})

	t.Run("stacks a mergeable item instead of adding a row", func(t *testing.T) {
		service, repo := build(catalog.GameItem{
			ItemIndex: 88888, HighGroup: 3, MiddleGroup: 1,
			IsSell: true, IsShopShow: true, PointPrice: 100, IsMerge: true,
		}, 5000, 5000)
		_, err := service.LoginBySteam(context.Background(), "stacker")
		assert.NoError(t, err)

		first, err := service.Buy(context.Background(), "stacker", 88888)
		assert.NoError(t, err)
		assert.True(t, first.IsNew)

		second, err := service.Buy(context.Background(), "stacker", 88888)
		assert.NoError(t, err)
		assert.False(t, second.IsNew)
		assert.Equal(t, uint32(2), second.Item.Value)

		stored, _, _ := repo.Find(context.Background(), "stacker")
		assert.Len(t, stored.Inventory, 1, "stacked, not a second row")
		assert.Equal(t, uint64(4800), stored.Point)
	})

	t.Run("rejects when funds are insufficient", func(t *testing.T) {
		service, _ := build(catalog.GameItem{
			ItemIndex: 99999, IsSell: true, IsShopShow: true, PointPrice: 999999,
		}, 100, 100)
		_, err := service.LoginBySteam(context.Background(), "poor")
		assert.NoError(t, err)

		_, err = service.Buy(context.Background(), "poor", 99999)
		assert.ErrorIs(t, err, services.ErrInsufficientFunds)
	})

	t.Run("rejects an item that is not for sale", func(t *testing.T) {
		service, _ := build(catalog.GameItem{
			ItemIndex: 99999, IsSell: false, IsShopShow: true, PointPrice: 10,
		}, 5000, 5000)
		_, err := service.LoginBySteam(context.Background(), "x")
		assert.NoError(t, err)

		_, err = service.Buy(context.Background(), "x", 99999)
		assert.ErrorIs(t, err, services.ErrItemNotForSale)
	})
}

func TestPlayerServiceCraft(t *testing.T) {
	recipe := catalog.Recipe{
		ID: 1, Exist: true, RequireLevel: 1, PricePoint: 100,
		Materials:  []catalog.RecipeMaterial{{ItemIndex: 500, Count: 2}},
		ItemOutput: 999,
	}
	output := catalog.GameItem{ItemIndex: 999, HighGroup: 2, MiddleGroup: 1}
	build := func() (*services.PlayerService, *repositories.MemoryPlayerRepository) {
		repo := repositories.NewMemoryPlayerRepository()
		gameCatalog := &catalog.Catalog{
			Items:   map[uint32]catalog.GameItem{output.ItemIndex: output},
			Recipes: map[uint32]catalog.Recipe{recipe.ID: recipe},
		}
		return services.NewPlayerService(repo, gameCatalog, 0, 0), repo
	}
	withMaterial := func(repo *repositories.MemoryPlayerRepository, steamID string, matValue uint32) {
		assert.NoError(t, repo.Create(context.Background(), models.Player{
			SteamID: steamID, Level: 1, Point: 5000,
			Inventory: []models.InventoryItem{{Serial: 10, ItemIndex: 500, Value: matValue, State: 1}},
		}))
	}

	t.Run("consumes materials + point and produces the output", func(t *testing.T) {
		service, repo := build()
		withMaterial(repo, "crafter", 5)

		result, err := service.Craft(context.Background(), "crafter", 1)
		assert.NoError(t, err)
		assert.Equal(t, uint32(999), result.Output.ItemIndex)
		assert.Equal(t, uint64(100), result.PricePoint)
		assert.Empty(t, result.Deleted)
		assert.Len(t, result.Updated, 1)

		stored, _, _ := repo.Find(context.Background(), "crafter")
		assert.Equal(t, uint64(4900), stored.Point)
		assert.Len(t, stored.Inventory, 2) // decremented material + new output
		mat, _ := stored.Item(10)
		assert.Equal(t, uint32(3), mat.Value)
	})

	t.Run("deletes a material that is fully consumed", func(t *testing.T) {
		service, repo := build()
		withMaterial(repo, "c2", 2)

		result, err := service.Craft(context.Background(), "c2", 1)
		assert.NoError(t, err)
		assert.Equal(t, []uint16{10}, result.Deleted)

		stored, _, _ := repo.Find(context.Background(), "c2")
		assert.Len(t, stored.Inventory, 1) // only the output remains
	})

	t.Run("rejects when materials are missing", func(t *testing.T) {
		service, repo := build()
		withMaterial(repo, "c3", 1) // needs 2, has 1

		_, err := service.Craft(context.Background(), "c3", 1)
		assert.ErrorIs(t, err, services.ErrMaterialMissing)
	})

	t.Run("rejects an unknown recipe", func(t *testing.T) {
		service, repo := build()
		assert.NoError(t, repo.Create(context.Background(), models.Player{SteamID: "c4", Level: 1}))

		_, err := service.Craft(context.Background(), "c4", 777)
		assert.ErrorIs(t, err, services.ErrRecipeUnknown)
	})
}

type failingPlayerRepository struct{}

func (failingPlayerRepository) Find(context.Context, string) (models.Player, bool, error) {
	return models.Player{}, false, errors.New("database unavailable")
}

func (failingPlayerRepository) FindMany(context.Context, []string) (map[string]models.Player, error) {
	return nil, errors.New("database unavailable")
}

func (failingPlayerRepository) Create(context.Context, models.Player) error {
	return errors.New("database unavailable")
}

func (failingPlayerRepository) Touch(context.Context, string, time.Time) error {
	return errors.New("database unavailable")
}

func (failingPlayerRepository) SaveLoadout(context.Context, string, models.Loadout) error {
	return errors.New("database unavailable")
}

func (failingPlayerRepository) CommitInventory(context.Context, string, []models.InventoryItem, models.Wallet, uint64) (bool, error) {
	return false, errors.New("database unavailable")
}

func (failingPlayerRepository) FindByName(context.Context, string) (models.Player, bool, error) {
	return models.Player{}, false, errors.New("database unavailable")
}

func (failingPlayerRepository) SaveSocial(context.Context, string, []models.Friend, []models.FriendRequest) error {
	return errors.New("database unavailable")
}

func (failingPlayerRepository) SetName(context.Context, string, string) error {
	return errors.New("database unavailable")
}

func (failingPlayerRepository) SetClanName(context.Context, string, string) error {
	return errors.New("database unavailable")
}

func (failingPlayerRepository) SaveAttachments(context.Context, string, models.Attachments) error {
	return errors.New("database unavailable")
}

func TestPlayerServiceChangeWeapons(t *testing.T) {
	newPlayer := func() models.Player {
		return models.Player{
			SteamID: "steam-1",
			Inventory: []models.InventoryItem{
				{Serial: 10, ItemIndex: 21102201, ItemType: models.SlotWeaponFirst},
				{Serial: 11, ItemIndex: 21102202, ItemType: models.SlotWeaponFirst + 1},
			},
			Loadout: models.Loadout{{Slot: models.SlotWeaponFirst, Serial: 10}},
		}
	}

	t.Run("swaps the whole set in one call and persists it", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), newPlayer()))

		updated, err := service.ChangeWeapons(context.Background(), "steam-1", []uint16{10, 11, 0, 0, 0})

		assert.NoError(t, err)
		assert.Len(t, updated.Loadout, 2)
		stored, _, _ := repo.Find(context.Background(), "steam-1")
		assert.Len(t, stored.Loadout, 2, "the change is persisted")
	})

	t.Run("each weapon lands in the slot its own type names", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), newPlayer()))

		updated, err := service.ChangeWeapons(context.Background(), "steam-1", []uint16{11, 0, 0, 0, 0})

		assert.NoError(t, err)
		assert.Contains(t, updated.Loadout, models.EquippedItem{Slot: models.SlotWeaponFirst + 1, Serial: 11})
	})

	t.Run("one bad serial leaves the loadout untouched", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), newPlayer()))

		_, err := service.ChangeWeapons(context.Background(), "steam-1", []uint16{11, 999, 0, 0, 0})

		assert.ErrorIs(t, err, services.ErrItemNotOwned)
		stored, _, _ := repo.Find(context.Background(), "steam-1")
		assert.Equal(t, uint16(10), stored.Loadout[0].Serial, "nothing was half applied")
	})

	t.Run("zero leaves a slot alone", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), newPlayer()))

		_, err := service.ChangeWeapons(context.Background(), "steam-1", []uint16{0, 0, 0, 0, 0})

		assert.NoError(t, err)
	})
}

// PartLinkTable.csv is the whitelist of which custom parts fit which weapon.
// The pairs below are real rows of that table, so the test fails if the catalog
// stops being loaded rather than passing on invented data.
func TestPlayerServicePartCompatibility(t *testing.T) {
	const (
		weaponIndex     = 21600101
		fittingPart     = 62400201
		notFittingPart  = 62400301
		weaponSerial    = 10
		partSerial      = 11
		otherPartSerial = 12
	)
	newPlayer := func() models.Player {
		return models.Player{
			SteamID: "steam-1",
			Point:   1000000,
			Cash:    1000000,
			Inventory: []models.InventoryItem{
				{Serial: weaponSerial, ItemIndex: weaponIndex, ItemType: models.SlotWeaponFirst},
				{Serial: partSerial, ItemIndex: fittingPart, ItemType: models.SlotCustomPartFirst},
				{Serial: otherPartSerial, ItemIndex: notFittingPart, ItemType: models.SlotCustomPartFirst},
			},
		}
	}

	t.Run("a part the table pairs with the weapon mounts", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), newPlayer()))

		updated, _, err := service.Attach(context.Background(), "steam-1", weaponSerial, partSerial)

		assert.NoError(t, err)
		assert.Len(t, updated.Attachments, 1)
	})

	t.Run("a part missing from the weapon's row is refused", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), newPlayer()))

		_, _, err := service.Attach(context.Background(), "steam-1", weaponSerial, otherPartSerial)

		assert.ErrorIs(t, err, services.ErrPartNotCompatible)
		stored, _, _ := repo.Find(context.Background(), "steam-1")
		assert.Empty(t, stored.Attachments, "a refused mount is not persisted")
	})

	t.Run("buying an incompatible part costs nothing", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		assert.NoError(t, repo.Create(context.Background(), newPlayer()))

		_, err := service.BuyCustomPart(context.Background(), "steam-1", weaponSerial, notFittingPart)

		assert.ErrorIs(t, err, services.ErrPartNotCompatible)
		stored, _, _ := repo.Find(context.Background(), "steam-1")
		assert.Equal(t, uint64(1000000), stored.Point, "the wallet was not touched")
		assert.Len(t, stored.Inventory, 3, "nothing was added to the inventory")
	})

	t.Run("gear and perks do not go through the weapon table", func(t *testing.T) {
		repo := repositories.NewMemoryPlayerRepository()
		service := newService(t, repo)
		player := newPlayer()
		player.Inventory = append(player.Inventory,
			models.InventoryItem{Serial: 20, ItemIndex: 99999999, ItemType: models.SlotPerkFirst})
		assert.NoError(t, repo.Create(context.Background(), player))

		_, _, err := service.Attach(context.Background(), "steam-1", weaponSerial, 20)

		assert.NoError(t, err, "a perk is not a custom part and has no link row")
	})
}
