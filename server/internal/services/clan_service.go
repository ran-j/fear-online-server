package services

import (
	"context"
	"errors"
	"time"

	"go-service-template/internal/models"
	"go-service-template/internal/repositories"
)

var (
	ErrClanNotFound  = errors.New("clan not found")
	ErrAlreadyMember = errors.New("already a member of this clan")
	ErrAlreadyInClan = errors.New("already in a clan")
)

type ClanService struct {
	clans   repositories.ClanRepository
	players repositories.PlayerRepository
}

func NewClanService(clans repositories.ClanRepository, players repositories.PlayerRepository) *ClanService {
	return &ClanService{clans: clans, players: players}
}

func (s *ClanService) NameAvailable(ctx context.Context, name string) (bool, error) {
	_, found, err := s.clans.Find(ctx, name)
	if err != nil {
		return false, err
	}
	return !found, nil
}

func (s *ClanService) PendingApplication(ctx context.Context, steamID string) (models.Clan, bool, error) {
	return s.clans.FindByApplicant(ctx, steamID)
}

func (s *ClanService) MarkAvailable(ctx context.Context, actor models.Player, mark [3]int32) (bool, error) {
	_, taken, err := s.clans.FindByMark(ctx, mark, actor.ClanName)
	if err != nil {
		return false, err
	}
	return !taken, nil
}

func (s *ClanService) ChangeMark(ctx context.Context, actor models.Player, mark [3]int32, markExtra uint16) (models.Clan, error) {
	clan, member, err := s.actorClan(ctx, actor)
	if err != nil {
		return models.Clan{}, err
	}
	if member.Rank != models.ClanRankMaster {
		return models.Clan{}, ErrNotAllowed
	}

	if _, taken, err := s.clans.FindByMark(ctx, mark, clan.Name); err != nil {
		return models.Clan{}, err
	} else if taken {
		return models.Clan{}, ErrMarkTaken
	}

	if err := s.clans.SaveMark(ctx, clan.Name, mark, markExtra); err != nil {
		return models.Clan{}, err
	}
	clan.Mark = mark
	clan.MarkExtra = markExtra
	return clan, nil
}

func (s *ClanService) Search(ctx context.Context, name string) ([]models.Clan, error) {
	return s.clans.SearchByName(ctx, name, 50)
}

func (s *ClanService) Find(ctx context.Context, name string) (models.Clan, bool, error) {
	return s.clans.Find(ctx, name)
}

func (s *ClanService) RequestJoin(ctx context.Context, player models.Player, clanName string) (models.Clan, error) {
	if player.ClanName != "" {
		return models.Clan{}, ErrAlreadyInClan
	}

	clan, found, err := s.clans.Find(ctx, clanName)
	if err != nil {
		return models.Clan{}, err
	}
	if !found {
		return models.Clan{}, ErrClanNotFound
	}
	for _, member := range clan.Members {
		if member.SteamID == player.SteamID {
			return models.Clan{}, ErrAlreadyMember
		}
	}
	for _, applicant := range clan.Applicants {
		if applicant.SteamID == player.SteamID {
			return clan, nil // already queued
		}
	}

	clan.Applicants = append(clan.Applicants, models.ClanApplicant{
		ApplicantID: nextApplicantID(clan),
		SteamID:     player.SteamID,
		Name:        player.Name,
		Level:       player.Level,
		AppliedAt:   time.Now().UTC(),
	})
	if err := s.clans.SaveApplicants(ctx, clan.Name, clan.Applicants); err != nil {
		return models.Clan{}, err
	}
	return clan, nil
}

func nextApplicantID(clan models.Clan) uint16 {
	highest := uint16(0)
	for _, member := range clan.Members {
		if member.MemberID > highest {
			highest = member.MemberID
		}
	}
	for _, applicant := range clan.Applicants {
		if applicant.ApplicantID > highest {
			highest = applicant.ApplicantID
		}
	}
	return highest + 1
}

func (s *ClanService) Create(ctx context.Context, steamID, playerName, clanName string, mark [3]int32, markExtra uint16) (models.Clan, error) {
	clan := models.Clan{
		Name:           clanName,
		Mark:           mark,
		MarkExtra:      markExtra,
		Level:          1,
		MemberCapacity: models.DefaultClanMemberCapacity,
		Master:         steamID,
		Members:        []models.ClanMember{{MemberID: 1, SteamID: steamID, Name: playerName, Rank: models.ClanRankMaster}},
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.clans.Create(ctx, clan); err != nil {
		return models.Clan{}, err
	}
	if err := s.players.SetClanName(ctx, steamID, clanName); err != nil {
		return models.Clan{}, err
	}
	return clan, nil
}
