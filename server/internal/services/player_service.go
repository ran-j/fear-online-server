package services

import (
	"context"
	"errors"
	"time"

	"go-service-template/internal/catalog"
	"go-service-template/internal/models"
	"go-service-template/internal/repositories"
)

var (
	ErrPlayerNotFound    = errors.New("player not found")
	ErrItemNotOwned      = errors.New("item not owned")
	ErrItemExpired       = errors.New("item expired")
	ErrPartNotCompatible = errors.New("part does not fit this weapon")
	ErrItemUnknown       = errors.New("item not in catalog")
	ErrItemNotForSale    = errors.New("item not for sale")
	ErrLevelTooLow       = errors.New("level requirement not met")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInventoryFull     = errors.New("inventory full")
	ErrRecipeUnknown     = errors.New("recipe not in catalog")
	ErrMaterialMissing   = errors.New("missing craft materials")
)

type PlayerService struct {
	players       repositories.PlayerRepository
	catalog       *catalog.Catalog
	startingPoint uint64
	startingCash  uint64
}

func NewPlayerService(players repositories.PlayerRepository, gameCatalog *catalog.Catalog, startingPoint, startingCash uint64) *PlayerService {
	return &PlayerService{
		players:       players,
		catalog:       gameCatalog,
		startingPoint: startingPoint,
		startingCash:  startingCash,
	}
}

func (s *PlayerService) LoginBySteam(ctx context.Context, steamID string) (models.Player, error) {
	now := time.Now().UTC()

	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return models.Player{}, err
	}
	if found {
		if err := s.players.Touch(ctx, steamID, now); err != nil {
			return models.Player{}, err
		}
		player.LastLogin = now
		// For now we only purge expired items on login, but we could also do it on any inventory mutation idk.
		loadoutChanged := len(player.PurgeExpired(now)) > 0
		if loadoutChanged {
			if _, err := s.players.CommitInventory(ctx, steamID, player.Inventory, models.WalletNone, 0); err != nil {
				return models.Player{}, err
			}
			if err := s.players.SaveAttachments(ctx, steamID, player.Attachments); err != nil {
				return models.Player{}, err
			}
		}
		equipped := make(map[uint8]bool)
		for _, entry := range player.Loadout {
			if item, ok := player.Item(entry.Serial); ok && item.ItemType == entry.Slot {
				equipped[entry.Slot] = true
			}
		}
		for _, item := range player.Inventory {
			if (item.ItemType == models.SlotCharacterATC || item.ItemType == models.SlotCharacterTF) && !equipped[item.ItemType] {
				player.Loadout = player.Loadout.Equip(item.ItemType, item.Serial)
				equipped[item.ItemType] = true
				loadoutChanged = true
			}
		}
		if loadoutChanged {
			if err := s.players.SaveLoadout(ctx, steamID, player.Loadout); err != nil {
				return models.Player{}, err
			}
		}
		return player, nil
	}

	newPlayer := s.newAccount(steamID)
	newPlayer.CreatedAt = now
	newPlayer.LastLogin = now
	if err := s.players.Create(ctx, newPlayer); err != nil {
		if errors.Is(err, repositories.ErrAlreadyExists) {
			existing, _, findErr := s.players.Find(ctx, steamID)
			return existing, findErr
		}
		return models.Player{}, err
	}
	return newPlayer, nil
}

func (s *PlayerService) FindMany(ctx context.Context, steamIDs []string) (map[string]models.Player, error) {
	return s.players.FindMany(ctx, steamIDs)
}

func (s *PlayerService) Equip(ctx context.Context, steamID string, serial uint16) (models.Player, error) {
	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return models.Player{}, err
	}
	if !found {
		return models.Player{}, ErrPlayerNotFound
	}

	item, ok := player.Item(serial)
	if !ok {
		return models.Player{}, ErrItemNotOwned
	}
	if item.IsExpired(time.Now().UTC()) {
		return models.Player{}, ErrItemExpired
	}
	player.Loadout = player.Loadout.Equip(item.ItemType, serial)

	if err := s.players.SaveLoadout(ctx, steamID, player.Loadout); err != nil {
		return models.Player{}, err
	}
	return player, nil
}

func (s *PlayerService) ChangeWeapons(ctx context.Context, steamID string, serials []uint16) (models.Player, error) {
	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return models.Player{}, err
	}
	if !found {
		return models.Player{}, ErrPlayerNotFound
	}

	now := time.Now().UTC()
	loadout := player.Loadout
	for _, serial := range serials {
		if serial == 0 {
			continue
		}
		item, ok := player.Item(serial)
		if !ok {
			return models.Player{}, ErrItemNotOwned
		}
		if item.IsExpired(now) {
			return models.Player{}, ErrItemExpired
		}
		loadout = loadout.Equip(item.ItemType, serial)
	}

	player.Loadout = loadout
	if err := s.players.SaveLoadout(ctx, steamID, player.Loadout); err != nil {
		return models.Player{}, err
	}
	return player, nil
}

func (s *PlayerService) Attach(ctx context.Context, steamID string, parentSerial, childSerial uint16) (models.Player, models.InventoryItem, error) {
	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return models.Player{}, models.InventoryItem{}, err
	}
	if !found {
		return models.Player{}, models.InventoryItem{}, ErrPlayerNotFound
	}

	child, ok := player.Item(childSerial)
	if !ok {
		return models.Player{}, models.InventoryItem{}, ErrItemNotOwned
	}
	parent, ok := player.Item(parentSerial)
	if !ok {
		return models.Player{}, models.InventoryItem{}, ErrItemNotOwned
	}
	if child.IsExpired(time.Now().UTC()) {
		return models.Player{}, models.InventoryItem{}, ErrItemExpired
	}
	if models.IsCustomSlot(child.ItemType) && !s.catalog.Accepts(parent.ItemIndex, child.ItemIndex) {
		return models.Player{}, models.InventoryItem{}, ErrPartNotCompatible
	}
	player.Attachments = player.Attachments.Attach(parentSerial, childSerial, child.ItemType)

	if err := s.players.SaveAttachments(ctx, steamID, player.Attachments); err != nil {
		return models.Player{}, models.InventoryItem{}, err
	}
	return player, child, nil
}

func (s *PlayerService) Detach(ctx context.Context, steamID string, parentSerial, childSerial uint16) (models.Player, models.InventoryItem, error) {
	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return models.Player{}, models.InventoryItem{}, err
	}
	if !found {
		return models.Player{}, models.InventoryItem{}, ErrPlayerNotFound
	}

	child, ok := player.Item(childSerial)
	if !ok {
		return models.Player{}, models.InventoryItem{}, ErrItemNotOwned
	}
	player.Attachments = player.Attachments.Detach(parentSerial, childSerial)

	if err := s.players.SaveAttachments(ctx, steamID, player.Attachments); err != nil {
		return models.Player{}, models.InventoryItem{}, err
	}
	return player, child, nil
}

type BuyResult struct {
	Player models.Player
	Item   models.InventoryItem
	IsNew  bool
	Wallet models.Wallet
	Price  uint64
}

func (s *PlayerService) Buy(ctx context.Context, steamID string, itemIndex uint32) (BuyResult, error) {
	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return BuyResult{}, err
	}
	if !found {
		return BuyResult{}, ErrPlayerNotFound
	}

	def, ok := s.catalog.Items[itemIndex]
	if !ok {
		return BuyResult{}, ErrItemUnknown
	}
	if !def.IsSell {
		return BuyResult{}, ErrItemNotForSale
	}
	if int(player.Level) < def.BuyLevelLimit {
		return BuyResult{}, ErrLevelTooLow
	}

	wallet, price := storePrice(def)
	if price > 0 && player.Balance(wallet) < price {
		return BuyResult{}, ErrInsufficientFunds
	}

	item, isNew := addToInventory(&player, def, itemIndex, time.Now().UTC())
	if isNew && item.Serial == 0 {
		return BuyResult{}, ErrInventoryFull
	}

	applied, err := s.players.CommitInventory(ctx, steamID, player.Inventory, wallet, price)
	if err != nil {
		return BuyResult{}, err
	}
	if !applied {
		return BuyResult{}, ErrInsufficientFunds // a concurrent spend won the race
	}
	player.Debit(wallet, price)

	return BuyResult{Player: player, Item: item, IsNew: isNew, Wallet: wallet, Price: price}, nil
}

func (s *PlayerService) BuyCustomPart(ctx context.Context, steamID string, weaponSerial uint16, itemIndex uint32) (BuyResult, error) {
	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return BuyResult{}, err
	}
	if !found {
		return BuyResult{}, ErrPlayerNotFound
	}
	weapon, ok := player.Item(weaponSerial)
	if !ok {
		return BuyResult{}, ErrItemNotOwned
	}
	if !s.catalog.Accepts(weapon.ItemIndex, itemIndex) {
		return BuyResult{}, ErrPartNotCompatible
	}

	result, err := s.Buy(ctx, steamID, itemIndex)
	if err != nil {
		return BuyResult{}, err
	}
	mounted, _, err := s.Attach(ctx, steamID, weaponSerial, result.Item.Serial)
	if err != nil {
		return BuyResult{}, err
	}
	result.Player = mounted
	return result, nil
}

type CraftResult struct {
	Player     models.Player
	Output     models.InventoryItem
	Deleted    []uint16               // serials of fully-consumed materials
	Updated    []models.InventoryItem // partially-consumed materials
	PricePoint uint64
}

func (s *PlayerService) Craft(ctx context.Context, steamID string, recipeID uint32) (CraftResult, error) {
	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return CraftResult{}, err
	}
	if !found {
		return CraftResult{}, ErrPlayerNotFound
	}

	recipe, ok := s.catalog.Recipes[recipeID]
	if !ok || !recipe.Exist {
		return CraftResult{}, ErrRecipeUnknown
	}
	if int(player.Level) < recipe.RequireLevel {
		return CraftResult{}, ErrLevelTooLow
	}
	if recipe.PricePoint > 0 && player.Point < recipe.PricePoint {
		return CraftResult{}, ErrInsufficientFunds
	}

	need := make(map[uint32]int, len(recipe.Materials))
	for _, material := range recipe.Materials {
		need[material.ItemIndex] += material.Count
	}
	deleted, updated, ok := player.ConsumeMaterials(need)
	if !ok {
		return CraftResult{}, ErrMaterialMissing
	}

	serial := player.NextSerial()
	if serial == 0 {
		return CraftResult{}, ErrInventoryFull
	}
	def := s.catalog.Items[recipe.ItemOutput]
	output := models.InventoryItem{
		Serial:    serial,
		ItemIndex: recipe.ItemOutput,
		Value:     1,
		State:     1,
		ItemType:  def.ItemType(),
		ExpiresAt: expiryFor(def, time.Now().UTC()),
	}
	player.Inventory = append(player.Inventory, output)

	applied, err := s.players.CommitInventory(ctx, steamID, player.Inventory, models.WalletPoint, recipe.PricePoint)
	if err != nil {
		return CraftResult{}, err
	}
	if !applied {
		return CraftResult{}, ErrInsufficientFunds
	}
	player.Debit(models.WalletPoint, recipe.PricePoint)

	return CraftResult{Player: player, Output: output, Deleted: deleted, Updated: updated, PricePoint: recipe.PricePoint}, nil
}

func addToInventory(player *models.Player, def catalog.GameItem, itemIndex uint32, now time.Time) (models.InventoryItem, bool) {
	if def.UseTime > 0 {
		for i := range player.Inventory {
			if player.Inventory[i].ItemIndex != itemIndex {
				continue
			}
			from := player.Inventory[i].ExpiresAt
			if from.Before(now) {
				from = now
			}
			player.Inventory[i].ExpiresAt = from.Add(time.Duration(def.UseTime) * time.Second)
			return player.Inventory[i], false
		}
	} else if def.IsMerge {
		for i := range player.Inventory {
			if player.Inventory[i].ItemIndex == itemIndex {
				player.Inventory[i].Value++
				return player.Inventory[i], false
			}
		}
	}

	item := models.InventoryItem{
		Serial:    player.NextSerial(),
		ItemIndex: itemIndex,
		Value:     1,
		State:     1,
		ItemType:  def.ItemType(),
		ExpiresAt: expiryFor(def, now),
	}
	if item.Serial != 0 {
		player.Inventory = append(player.Inventory, item)
	}
	return item, true
}

func storePrice(def catalog.GameItem) (models.Wallet, uint64) {
	pointPrice := def.PointPrice
	if def.IsSale && def.SalePrice > 0 {
		pointPrice = def.SalePrice
	}
	switch {
	case def.CashType == 1 && def.CashPrice > 0:
		return models.WalletCash, def.CashPrice
	case def.CashType != 1 && pointPrice > 0:
		return models.WalletPoint, pointPrice
	case def.CashPrice > 0:
		return models.WalletCash, def.CashPrice
	default:
		return models.WalletNone, 0
	}
}

func (s *PlayerService) FindByName(ctx context.Context, name string) (models.Player, bool, error) {
	return s.players.FindByName(ctx, name)
}

func (s *PlayerService) Rename(ctx context.Context, steamID, name string) (models.Player, error) {
	if err := s.players.SetName(ctx, steamID, name); err != nil {
		return models.Player{}, err
	}
	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return models.Player{}, err
	}
	if !found {
		return models.Player{}, ErrPlayerNotFound
	}
	return player, nil
}

func expiryFor(def catalog.GameItem, now time.Time) time.Time {
	if def.UseTime <= 0 {
		return time.Time{}
	}
	return now.Add(time.Duration(def.UseTime) * time.Second)
}

func (s *PlayerService) newAccount(steamID string) models.Player {
	player := models.Player{
		SteamID: steamID,
		Name:    placeholderName(steamID),
		Level:   1,
		Point:   s.startingPoint,
		Cash:    s.startingCash,
	}

	now := time.Now().UTC()
	serial := uint16(1)
	for _, itemIndex := range catalog.StarterItems {
		item, ok := s.catalog.Items[itemIndex]
		if !ok {
			continue
		}
		itemType := item.ItemType()
		player.Inventory = append(player.Inventory, models.InventoryItem{
			Serial:    serial,
			ItemIndex: itemIndex,
			Value:     1,
			State:     1,
			ItemType:  itemType,
			ExpiresAt: expiryFor(item, now),
		})
		player.Loadout = append(player.Loadout, models.EquippedItem{Slot: itemType, Serial: serial})
		serial++
	}
	return player
}

func placeholderName(steamID string) string {
	if len(steamID) >= 5 {
		return "Player" + steamID[len(steamID)-5:]
	}
	return "Player" + steamID
}
