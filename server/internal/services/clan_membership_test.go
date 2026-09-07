package services_test

import (
	"context"
	"testing"

	"go-service-template/internal/models"
	"go-service-template/internal/repositories"
	"go-service-template/internal/services"

	"github.com/stretchr/testify/assert"
)

const (
	masterID    = "steam-master"
	managerID   = "steam-manager"
	memberID    = "steam-member"
	applicantID = "steam-applicant"
)

// clanFixture builds a clan with a master (1), a manager (2), a plain member (3)
// and one pending applicant (4), plus the accounts behind them. Extra members
// let a case add the position it needs to exercise.
func clanFixture(t *testing.T, extra ...models.ClanMember) (*services.ClanService, *repositories.MemoryPlayerRepository, models.Clan) {
	t.Helper()

	players := repositories.NewMemoryPlayerRepository()
	clans := repositories.NewMemoryClanRepository()
	ctx := context.Background()

	for steamID, name := range map[string]string{
		masterID: "Master", managerID: "Manager", memberID: "Member", applicantID: "Applicant",
	} {
		clanName := "Reapers"
		if steamID == applicantID {
			clanName = ""
		}
		assert.NoError(t, players.Create(ctx, models.Player{SteamID: steamID, Name: name, ClanName: clanName}))
	}

	clan := models.Clan{
		Name:           "Reapers",
		Master:         masterID,
		Level:          1,
		MemberCapacity: models.DefaultClanMemberCapacity,
		Members: []models.ClanMember{
			{MemberID: 1, SteamID: masterID, Name: "Master", Rank: models.ClanRankMaster},
			{MemberID: 2, SteamID: managerID, Name: "Manager", Rank: models.ClanRankManager},
			{MemberID: 3, SteamID: memberID, Name: "Member", Rank: models.ClanRankMember},
		},
		Applicants: []models.ClanApplicant{
			{ApplicantID: 4, SteamID: applicantID, Name: "Applicant"},
		},
	}
	clan.Members = append(clan.Members, extra...)
	assert.NoError(t, clans.Create(ctx, clan))
	return services.NewClanService(clans, players), players, clan
}

func actor(t *testing.T, players *repositories.MemoryPlayerRepository, steamID string) models.Player {
	t.Helper()
	player, found, err := players.Find(context.Background(), steamID)
	assert.NoError(t, err)
	assert.True(t, found)
	return player
}

func TestAcceptApplication(t *testing.T) {
	t.Run("enrols the applicant keeping their id and links the account", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		clan, member, err := service.AcceptApplication(context.Background(), actor(t, players, masterID), 4)

		assert.NoError(t, err)
		assert.Equal(t, uint16(4), member.MemberID)
		assert.Equal(t, models.ClanRankMember, member.Rank)
		assert.Len(t, clan.Members, 4)
		assert.Empty(t, clan.Applicants)
		assert.Equal(t, "Reapers", actor(t, players, applicantID).ClanName)
	})

	t.Run("a plain member cannot accept", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		_, _, err := service.AcceptApplication(context.Background(), actor(t, players, memberID), 4)

		assert.ErrorIs(t, err, services.ErrNotAllowed)
		assert.Empty(t, actor(t, players, applicantID).ClanName)
	})

	t.Run("an unknown applicant id is rejected", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		_, _, err := service.AcceptApplication(context.Background(), actor(t, players, masterID), 99)

		assert.ErrorIs(t, err, services.ErrApplicantNotFound)
	})
}

func TestCancelAndRejectApplication(t *testing.T) {
	t.Run("the applicant withdraws their own application", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		clan, applicant, err := service.CancelApplication(context.Background(), actor(t, players, applicantID))

		assert.NoError(t, err)
		assert.Equal(t, uint16(4), applicant.ApplicantID)
		assert.Empty(t, clan.Applicants)
	})

	t.Run("cancelling without a pending application fails", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		_, _, err := service.CancelApplication(context.Background(), actor(t, players, memberID))

		assert.ErrorIs(t, err, services.ErrNoApplication)
	})

	t.Run("a manager rejects without linking the account", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		clan, _, err := service.RejectApplication(context.Background(), actor(t, players, managerID), 4)

		assert.NoError(t, err)
		assert.Empty(t, clan.Applicants)
		assert.Len(t, clan.Members, 3)
		assert.Empty(t, actor(t, players, applicantID).ClanName)
	})
}

func TestKickMember(t *testing.T) {
	t.Run("a manager kicks a plain member and unlinks the account", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		clan, member, err := service.KickMember(context.Background(), actor(t, players, managerID), 3)

		assert.NoError(t, err)
		assert.Equal(t, "Member", member.Name)
		assert.Len(t, clan.Members, 2)
		assert.Empty(t, actor(t, players, memberID).ClanName)
	})

	t.Run("the master cannot be kicked", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		_, _, err := service.KickMember(context.Background(), actor(t, players, managerID), 1)

		assert.ErrorIs(t, err, services.ErrNotAllowed)
	})

	t.Run("a manager cannot kick another manager", func(t *testing.T) {
		service, players, _ := clanFixture(t, models.ClanMember{
			MemberID: 5, SteamID: "steam-other", Name: "Other", Rank: models.ClanRankManager,
		})

		_, _, err := service.KickMember(context.Background(), actor(t, players, managerID), 5)

		assert.ErrorIs(t, err, services.ErrNotAllowed)
	})
}

func TestLeaveClan(t *testing.T) {
	t.Run("a member leaves on their own", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		clan, member, err := service.Leave(context.Background(), actor(t, players, memberID))

		assert.NoError(t, err)
		assert.Equal(t, uint16(3), member.MemberID)
		assert.Len(t, clan.Members, 2)
		assert.Empty(t, actor(t, players, memberID).ClanName)
	})

	t.Run("the master has to hand the clan over first", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		_, _, err := service.Leave(context.Background(), actor(t, players, masterID))

		assert.ErrorIs(t, err, services.ErrMasterMustTransfer)
		assert.Equal(t, "Reapers", actor(t, players, masterID).ClanName)
	})
}

func TestChangePosition(t *testing.T) {
	t.Run("the master promotes a member to manager", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		clan, member, err := service.ChangePosition(context.Background(), actor(t, players, masterID), 3, models.ClanRankManager)

		assert.NoError(t, err)
		assert.Equal(t, models.ClanRankManager, member.Rank)
		promoted, _ := clan.MemberByID(3)
		assert.Equal(t, models.ClanRankManager, promoted.Rank)
	})

	t.Run("master cannot be handed out as a position", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		_, _, err := service.ChangePosition(context.Background(), actor(t, players, masterID), 3, models.ClanRankMaster)

		assert.ErrorIs(t, err, services.ErrInvalidRank)
	})

	t.Run("a manager cannot assign positions", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		_, _, err := service.ChangePosition(context.Background(), actor(t, players, managerID), 3, models.ClanRankAssociate)

		assert.ErrorIs(t, err, services.ErrNotAllowed)
	})
}

func TestClanMark(t *testing.T) {
	newMark := [3]int32{7, 8, 9}

	t.Run("the master repaints the emblem", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		clan, err := service.ChangeMark(context.Background(), actor(t, players, masterID), newMark, 4)

		assert.NoError(t, err)
		assert.Equal(t, newMark, clan.Mark)
		assert.Equal(t, uint16(4), clan.MarkExtra)

		stored, found, err := service.Find(context.Background(), "Reapers")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, newMark, stored.Mark)
	})

	t.Run("a manager cannot repaint", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		_, err := service.ChangeMark(context.Background(), actor(t, players, managerID), newMark, 0)

		assert.ErrorIs(t, err, services.ErrNotAllowed)
	})

	t.Run("an emblem worn by another clan is refused", func(t *testing.T) {
		service, players, _ := clanFixture(t)
		_, err := service.Create(context.Background(), "steam-rival", "Rival", "Wolves", newMark, 0)
		assert.NoError(t, err)

		available, err := service.MarkAvailable(context.Background(), actor(t, players, masterID), newMark)
		assert.NoError(t, err)
		assert.False(t, available)

		_, err = service.ChangeMark(context.Background(), actor(t, players, masterID), newMark, 0)
		assert.ErrorIs(t, err, services.ErrMarkTaken)
	})

	t.Run("the clan's own current emblem stays available to itself", func(t *testing.T) {
		service, players, clan := clanFixture(t)

		available, err := service.MarkAvailable(context.Background(), actor(t, players, masterID), clan.Mark)

		assert.NoError(t, err)
		assert.True(t, available)
	})
}

func TestDisband(t *testing.T) {
	t.Run("the master deletes the clan and unlinks every member", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		clan, err := service.Disband(context.Background(), actor(t, players, masterID))

		assert.NoError(t, err)
		assert.Len(t, clan.Members, 3, "the returned clan still lists who has to be notified")
		for _, steamID := range []string{masterID, managerID, memberID} {
			assert.Empty(t, actor(t, players, steamID).ClanName, steamID)
		}

		_, found, err := service.Find(context.Background(), "Reapers")
		assert.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("a manager cannot delete the clan", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		_, err := service.Disband(context.Background(), actor(t, players, managerID))

		assert.ErrorIs(t, err, services.ErrNotAllowed)
		_, found, _ := service.Find(context.Background(), "Reapers")
		assert.True(t, found)
	})
}

func TestChangeMaster(t *testing.T) {
	t.Run("the outgoing master stays on as a manager", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		clan, successor, previous, err := service.ChangeMaster(context.Background(), actor(t, players, masterID), 3)

		assert.NoError(t, err)
		assert.Equal(t, memberID, clan.Master)
		assert.Equal(t, models.ClanRankMaster, successor.Rank)
		assert.Equal(t, models.ClanRankManager, previous.Rank)
	})

	t.Run("only the master hands the clan over", func(t *testing.T) {
		service, players, _ := clanFixture(t)

		_, _, _, err := service.ChangeMaster(context.Background(), actor(t, players, managerID), 3)

		assert.ErrorIs(t, err, services.ErrNotAllowed)
	})
}

func TestSpendConsumable(t *testing.T) {
	// Both Clan_Mark_Change variants share FunctionIndex 1901: one is bought
	// with cash, the other is handed out for free.
	const (
		clanMarkChangeFree = 44510501
		clanMarkChangePaid = 44510502
	)

	newPlayerWith := func(t *testing.T, itemIndex uint32, count uint32) (*services.PlayerService, *repositories.MemoryPlayerRepository) {
		t.Helper()
		players := repositories.NewMemoryPlayerRepository()
		service := newService(t, players)
		player, err := service.LoginBySteam(context.Background(), "55555555555555555")
		assert.NoError(t, err)
		player.Inventory = append(player.Inventory, models.InventoryItem{
			Serial: 99, ItemIndex: itemIndex, Value: count, State: 1,
		})
		_, err = players.CommitInventory(context.Background(), player.SteamID, player.Inventory, models.WalletNone, 0)
		assert.NoError(t, err)
		return service, players
	}

	newPlayerWithStack := func(t *testing.T, count uint32) (*services.PlayerService, *repositories.MemoryPlayerRepository) {
		t.Helper()
		return newPlayerWith(t, clanMarkChangePaid, count)
	}

	t.Run("either variant pays for the change", func(t *testing.T) {
		for _, itemIndex := range []uint32{clanMarkChangeFree, clanMarkChangePaid} {
			service, repo := newPlayerWith(t, itemIndex, 1)

			player, found, err := repo.Find(context.Background(), "55555555555555555")
			assert.NoError(t, err)
			assert.True(t, found)

			item, owns := service.FindConsumable(player, services.FunctionClanMarkChange)
			assert.True(t, owns, "item %d should pay for a mark change", itemIndex)
			assert.Equal(t, itemIndex, item.ItemIndex)
		}
	})

	t.Run("a single-use stack leaves the inventory", func(t *testing.T) {
		service, _ := newPlayerWithStack(t, 1)

		player, spent, err := service.SpendConsumable(context.Background(), "55555555555555555", services.FunctionClanMarkChange)

		assert.NoError(t, err)
		assert.True(t, spent.UsedUp)
		assert.Equal(t, uint16(99), spent.Serial)
		_, still := player.Item(99)
		assert.False(t, still)
	})

	t.Run("a bigger stack just loses one unit", func(t *testing.T) {
		service, _ := newPlayerWithStack(t, 3)

		_, spent, err := service.SpendConsumable(context.Background(), "55555555555555555", services.FunctionClanMarkChange)

		assert.NoError(t, err)
		assert.False(t, spent.UsedUp)
		assert.Equal(t, uint32(2), spent.Remaining.Value)
	})

	t.Run("without the item the spend is refused", func(t *testing.T) {
		service := newService(t, repositories.NewMemoryPlayerRepository())
		_, err := service.LoginBySteam(context.Background(), "55555555555555555")
		assert.NoError(t, err)

		_, _, err = service.SpendConsumable(context.Background(), "55555555555555555", services.FunctionClanMarkChange)

		assert.ErrorIs(t, err, services.ErrConsumableMissing)
	})
}
