package services

import (
	"context"
	"errors"

	"go-service-template/internal/models"
)

var ErrConsumableMissing = errors.New("no such consumable in inventory")

// FunctionClanMarkChange is the catalog FunctionIndex shared by every item that
// pays for repainting a clan emblem, free or cash. Matching on the function
// rather than on one ItemIndex keeps every variant working.
const FunctionClanMarkChange = 1901

func (s *PlayerService) ItemFunctionType(itemIndex uint32) byte {
	return s.catalog.FunctionType(itemIndex)
}

func (s *PlayerService) FindConsumable(player models.Player, functionIndex int) (models.InventoryItem, bool) {
	for _, item := range player.Inventory {
		def, known := s.catalog.Items[item.ItemIndex]
		if known && def.FunctionIndex == functionIndex && item.Value > 0 {
			return item, true
		}
	}
	return models.InventoryItem{}, false
}

func (s *PlayerService) SpendConsumable(ctx context.Context, steamID string, functionIndex int) (models.Player, models.SpentItem, error) {
	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return models.Player{}, models.SpentItem{}, err
	}
	if !found {
		return models.Player{}, models.SpentItem{}, ErrPlayerNotFound
	}

	item, ok := s.FindConsumable(player, functionIndex)
	if !ok {
		return models.Player{}, models.SpentItem{}, ErrConsumableMissing
	}
	spent, ok := player.SpendUnit(item.Serial)
	if !ok {
		return models.Player{}, models.SpentItem{}, ErrConsumableMissing
	}

	if _, err := s.players.CommitInventory(ctx, steamID, player.Inventory, models.WalletNone, 0); err != nil {
		return models.Player{}, models.SpentItem{}, err
	}
	return player, spent, nil
}
