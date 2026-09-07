package services_test

import (
	"context"
	"testing"

	"go-service-template/internal/models"
	"go-service-template/internal/repositories"
	"go-service-template/internal/services"

	"github.com/stretchr/testify/assert"
)

func friendFixture(t *testing.T) (*services.FriendService, *repositories.MemoryPlayerRepository) {
	t.Helper()
	players := repositories.NewMemoryPlayerRepository()
	ctx := context.Background()
	for steamID, name := range map[string]string{"steam-a": "Ayla", "steam-b": "Boro"} {
		assert.NoError(t, players.Create(ctx, models.Player{SteamID: steamID, Name: name}))
	}
	return services.NewFriendService(players), players
}

func account(t *testing.T, players *repositories.MemoryPlayerRepository, steamID string) models.Player {
	t.Helper()
	player, found, err := players.Find(context.Background(), steamID)
	assert.NoError(t, err)
	assert.True(t, found)
	return player
}

func TestFriendRequest(t *testing.T) {
	t.Run("queues on the target, not the sender", func(t *testing.T) {
		service, players := friendFixture(t)

		target, request, err := service.Request(context.Background(), "steam-a", "Boro")

		assert.NoError(t, err)
		assert.Equal(t, "steam-b", target.SteamID)
		assert.Equal(t, "steam-a", request.SteamID)
		assert.Len(t, account(t, players, "steam-b").FriendRequests, 1)
		assert.Empty(t, account(t, players, "steam-a").FriendRequests, "the sender keeps no copy")
	})

	t.Run("asking twice does not stack rows", func(t *testing.T) {
		service, players := friendFixture(t)
		ayla := account(t, players, "steam-a")

		_, _, err := service.Request(context.Background(), ayla.SteamID, "Boro")
		assert.NoError(t, err)
		_, _, err = service.Request(context.Background(), ayla.SteamID, "Boro")

		assert.ErrorIs(t, err, services.ErrRequestPending)
		assert.Len(t, account(t, players, "steam-b").FriendRequests, 1)
	})

	t.Run("an unknown name and yourself are refused", func(t *testing.T) {
		service, players := friendFixture(t)
		ayla := account(t, players, "steam-a")

		_, _, err := service.Request(context.Background(), ayla.SteamID, "Nobody")
		assert.ErrorIs(t, err, services.ErrFriendNotFound)

		_, _, err = service.Request(context.Background(), ayla.SteamID, "Ayla")
		assert.ErrorIs(t, err, services.ErrFriendSelf)
	})
}

func TestFriendAcceptAndDelete(t *testing.T) {
	settle := func(t *testing.T) (*services.FriendService, *repositories.MemoryPlayerRepository) {
		t.Helper()
		service, players := friendFixture(t)
		_, request, err := service.Request(context.Background(), "steam-a", "Boro")
		assert.NoError(t, err)
		_, _, _, err = service.Accept(context.Background(), "steam-b", request.RequestID)
		assert.NoError(t, err)
		return service, players
	}

	t.Run("accepting adds both sides and clears the queue", func(t *testing.T) {
		_, players := settle(t)

		boro := account(t, players, "steam-b")
		assert.Len(t, boro.Friends, 1)
		assert.Empty(t, boro.FriendRequests)
		assert.True(t, boro.IsFriendOf("steam-a"))
		assert.True(t, account(t, players, "steam-a").IsFriendOf("steam-b"), "the sender gets the mirror entry")
	})

	t.Run("an unknown request id is refused", func(t *testing.T) {
		service, _ := friendFixture(t)

		_, _, _, err := service.Accept(context.Background(), "steam-b", 99)

		assert.ErrorIs(t, err, services.ErrRequestNotFound)
	})

	t.Run("deleting unfriends both ways", func(t *testing.T) {
		service, players := settle(t)
		boro := account(t, players, "steam-b")

		_, mirrored, err := service.Delete(context.Background(), boro.SteamID, boro.Friends[0].FriendID)

		assert.NoError(t, err)
		assert.NotZero(t, mirrored.FriendID, "the other side was found and removed")
		assert.Empty(t, account(t, players, "steam-b").Friends)
		assert.Empty(t, account(t, players, "steam-a").Friends)
	})
}

func TestFriendReject(t *testing.T) {
	service, players := friendFixture(t)
	_, request, err := service.Request(context.Background(), "steam-a", "Boro")
	assert.NoError(t, err)

	_, err = service.Reject(context.Background(), "steam-b", request.RequestID)

	assert.NoError(t, err)
	assert.Empty(t, account(t, players, "steam-b").FriendRequests)
	assert.Empty(t, account(t, players, "steam-a").Friends, "rejecting befriends nobody")
}
