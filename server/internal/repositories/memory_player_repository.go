package repositories

import (
	"context"
	"sync"
	"time"

	"go-service-template/internal/models"
)

type MemoryPlayerRepository struct {
	mu      sync.Mutex
	players map[string]models.Player
}

func NewMemoryPlayerRepository() *MemoryPlayerRepository {
	return &MemoryPlayerRepository{players: make(map[string]models.Player)}
}

func (r *MemoryPlayerRepository) Find(_ context.Context, steamID string) (models.Player, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	player, ok := r.players[steamID]
	return player, ok, nil
}

func (r *MemoryPlayerRepository) FindMany(_ context.Context, steamIDs []string) (map[string]models.Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	found := make(map[string]models.Player, len(steamIDs))
	for _, steamID := range steamIDs {
		if player, ok := r.players[steamID]; ok {
			found[steamID] = player
		}
	}
	return found, nil
}

func (r *MemoryPlayerRepository) Create(_ context.Context, player models.Player) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.players[player.SteamID]; ok {
		return ErrAlreadyExists
	}
	r.players[player.SteamID] = player
	return nil
}

func (r *MemoryPlayerRepository) Touch(_ context.Context, steamID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if player, ok := r.players[steamID]; ok {
		player.LastLogin = at
		r.players[steamID] = player
	}
	return nil
}

func (r *MemoryPlayerRepository) SaveLoadout(_ context.Context, steamID string, loadout models.Loadout) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if player, ok := r.players[steamID]; ok {
		player.Loadout = loadout
		r.players[steamID] = player
	}
	return nil
}

func (r *MemoryPlayerRepository) SaveAttachments(_ context.Context, steamID string, attachments models.Attachments) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if player, ok := r.players[steamID]; ok {
		player.Attachments = attachments
		r.players[steamID] = player
	}
	return nil
}

func (r *MemoryPlayerRepository) FindByName(_ context.Context, name string) (models.Player, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, player := range r.players {
		if player.Name == name {
			return player, true, nil
		}
	}
	return models.Player{}, false, nil
}

func (r *MemoryPlayerRepository) SaveSocial(_ context.Context, steamID string, friends []models.Friend, requests []models.FriendRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if player, ok := r.players[steamID]; ok {
		player.Friends = friends
		player.FriendRequests = requests
		r.players[steamID] = player
	}
	return nil
}

func (r *MemoryPlayerRepository) SetName(_ context.Context, steamID, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if player, ok := r.players[steamID]; ok {
		player.Name = name
		r.players[steamID] = player
	}
	return nil
}

func (r *MemoryPlayerRepository) SetClanName(_ context.Context, steamID, clanName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if player, ok := r.players[steamID]; ok {
		player.ClanName = clanName
		r.players[steamID] = player
	}
	return nil
}

func (r *MemoryPlayerRepository) CommitInventory(_ context.Context, steamID string, inventory []models.InventoryItem, wallet models.Wallet, price uint64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	player, ok := r.players[steamID]
	if !ok {
		return false, nil
	}
	if price > 0 && player.Balance(wallet) < price {
		return false, nil
	}
	player.Debit(wallet, price)
	player.Inventory = inventory
	r.players[steamID] = player
	return true, nil
}
