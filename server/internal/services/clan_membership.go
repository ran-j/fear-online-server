package services

import (
	"context"
	"errors"

	"go-service-template/internal/models"
)

var (
	ErrNotInClan          = errors.New("not in a clan")
	ErrNotAllowed         = errors.New("insufficient clan position")
	ErrMemberNotFound     = errors.New("clan member not found")
	ErrApplicantNotFound  = errors.New("clan applicant not found")
	ErrClanFull           = errors.New("clan is full")
	ErrNoApplication      = errors.New("no pending application")
	ErrInvalidRank        = errors.New("position cannot be assigned")
	ErrMasterMustTransfer = errors.New("master must hand over the clan before leaving")
	ErrMarkTaken          = errors.New("another clan already wears this emblem")
)

func (s *ClanService) AcceptApplication(ctx context.Context, actor models.Player, applicantID uint16) (models.Clan, models.ClanMember, error) {
	clan, _, err := s.reviewerClan(ctx, actor)
	if err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}

	applicant, ok := clan.RemoveApplicant(applicantID)
	if !ok {
		return models.Clan{}, models.ClanMember{}, ErrApplicantNotFound
	}
	if clan.IsFull() {
		return models.Clan{}, models.ClanMember{}, ErrClanFull
	}

	member := models.ClanMember{
		MemberID: applicant.ApplicantID,
		SteamID:  applicant.SteamID,
		Name:     applicant.Name,
		Rank:     models.ClanRankMember,
	}
	clan.Members = append(clan.Members, member)

	if err := s.clans.SaveRoster(ctx, clan.Name, clan.Members, clan.Applicants); err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}
	if err := s.players.SetClanName(ctx, member.SteamID, clan.Name); err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}
	return clan, member, nil
}

func (s *ClanService) RejectApplication(ctx context.Context, actor models.Player, applicantID uint16) (models.Clan, models.ClanApplicant, error) {
	clan, _, err := s.reviewerClan(ctx, actor)
	if err != nil {
		return models.Clan{}, models.ClanApplicant{}, err
	}

	applicant, ok := clan.RemoveApplicant(applicantID)
	if !ok {
		return models.Clan{}, models.ClanApplicant{}, ErrApplicantNotFound
	}
	if err := s.clans.SaveApplicants(ctx, clan.Name, clan.Applicants); err != nil {
		return models.Clan{}, models.ClanApplicant{}, err
	}
	return clan, applicant, nil
}

func (s *ClanService) CancelApplication(ctx context.Context, actor models.Player) (models.Clan, models.ClanApplicant, error) {
	clan, found, err := s.clans.FindByApplicant(ctx, actor.SteamID)
	if err != nil {
		return models.Clan{}, models.ClanApplicant{}, err
	}
	if !found {
		return models.Clan{}, models.ClanApplicant{}, ErrNoApplication
	}

	var withdrawn models.ClanApplicant
	for _, applicant := range clan.Applicants {
		if applicant.SteamID == actor.SteamID {
			withdrawn = applicant
			break
		}
	}
	clan.RemoveApplicant(withdrawn.ApplicantID)

	if err := s.clans.SaveApplicants(ctx, clan.Name, clan.Applicants); err != nil {
		return models.Clan{}, models.ClanApplicant{}, err
	}
	return clan, withdrawn, nil
}

func (s *ClanService) KickMember(ctx context.Context, actor models.Player, memberID uint16) (models.Clan, models.ClanMember, error) {
	clan, actorMember, err := s.reviewerClan(ctx, actor)
	if err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}

	target, ok := clan.MemberByID(memberID)
	if !ok {
		return models.Clan{}, models.ClanMember{}, ErrMemberNotFound
	}
	// The master answers to nobody, and leaving is its own request.
	if target.Rank == models.ClanRankMaster || target.MemberID == actorMember.MemberID {
		return models.Clan{}, models.ClanMember{}, ErrNotAllowed
	}
	// A manager may only remove ranks below their own.
	if actorMember.Rank == models.ClanRankManager && target.Rank == models.ClanRankManager {
		return models.Clan{}, models.ClanMember{}, ErrNotAllowed
	}

	clan.RemoveMember(memberID)
	if err := s.detach(ctx, clan, target); err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}
	return clan, target, nil
}

func (s *ClanService) Leave(ctx context.Context, actor models.Player) (models.Clan, models.ClanMember, error) {
	clan, member, err := s.actorClan(ctx, actor)
	if err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}
	if member.Rank == models.ClanRankMaster && len(clan.Members) > 1 {
		return models.Clan{}, models.ClanMember{}, ErrMasterMustTransfer
	}

	clan.RemoveMember(member.MemberID)
	if err := s.detach(ctx, clan, member); err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}
	return clan, member, nil
}

func (s *ClanService) ChangePosition(ctx context.Context, actor models.Player, memberID uint16, rank uint8) (models.Clan, models.ClanMember, error) {
	clan, actorMember, err := s.actorClan(ctx, actor)
	if err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}
	if actorMember.Rank != models.ClanRankMaster {
		return models.Clan{}, models.ClanMember{}, ErrNotAllowed
	}
	if !models.IsAssignableClanRank(rank) {
		return models.Clan{}, models.ClanMember{}, ErrInvalidRank
	}

	target, ok := clan.MemberByID(memberID)
	if !ok {
		return models.Clan{}, models.ClanMember{}, ErrMemberNotFound
	}
	if target.MemberID == actorMember.MemberID {
		return models.Clan{}, models.ClanMember{}, ErrNotAllowed
	}

	clan.SetRank(memberID, rank)
	target.Rank = rank
	if err := s.clans.SaveMembers(ctx, clan.Name, clan.Members); err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}
	return clan, target, nil
}

func (s *ClanService) ChangeMaster(ctx context.Context, actor models.Player, memberID uint16) (models.Clan, models.ClanMember, models.ClanMember, error) {
	clan, actorMember, err := s.actorClan(ctx, actor)
	if err != nil {
		return models.Clan{}, models.ClanMember{}, models.ClanMember{}, err
	}
	if actorMember.Rank != models.ClanRankMaster {
		return models.Clan{}, models.ClanMember{}, models.ClanMember{}, ErrNotAllowed
	}

	target, ok := clan.MemberByID(memberID)
	if !ok || target.MemberID == actorMember.MemberID {
		return models.Clan{}, models.ClanMember{}, models.ClanMember{}, ErrMemberNotFound
	}

	clan.SetRank(target.MemberID, models.ClanRankMaster)
	clan.SetRank(actorMember.MemberID, models.ClanRankManager)
	clan.Master = target.SteamID
	target.Rank = models.ClanRankMaster
	actorMember.Rank = models.ClanRankManager

	if err := s.clans.SaveLeadership(ctx, clan.Name, clan.Master, clan.Members); err != nil {
		return models.Clan{}, models.ClanMember{}, models.ClanMember{}, err
	}
	return clan, target, actorMember, nil
}

func (s *ClanService) Disband(ctx context.Context, actor models.Player) (models.Clan, error) {
	clan, member, err := s.actorClan(ctx, actor)
	if err != nil {
		return models.Clan{}, err
	}
	if member.Rank != models.ClanRankMaster {
		return models.Clan{}, ErrNotAllowed
	}

	if err := s.clans.Delete(ctx, clan.Name); err != nil {
		return models.Clan{}, err
	}
	for _, enrolled := range clan.Members {
		if err := s.players.SetClanName(ctx, enrolled.SteamID, ""); err != nil {
			return models.Clan{}, err
		}
	}
	return clan, nil
}

func (s *ClanService) detach(ctx context.Context, clan models.Clan, member models.ClanMember) error {
	if err := s.clans.SaveMembers(ctx, clan.Name, clan.Members); err != nil {
		return err
	}
	return s.players.SetClanName(ctx, member.SteamID, "")
}

func (s *ClanService) actorClan(ctx context.Context, actor models.Player) (models.Clan, models.ClanMember, error) {
	if actor.ClanName == "" {
		return models.Clan{}, models.ClanMember{}, ErrNotInClan
	}

	clan, found, err := s.clans.Find(ctx, actor.ClanName)
	if err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}
	if !found {
		return models.Clan{}, models.ClanMember{}, ErrClanNotFound
	}

	member, ok := clan.Member(actor.SteamID)
	if !ok {
		return models.Clan{}, models.ClanMember{}, ErrNotInClan
	}
	return clan, member, nil
}

func (s *ClanService) reviewerClan(ctx context.Context, actor models.Player) (models.Clan, models.ClanMember, error) {
	clan, member, err := s.actorClan(ctx, actor)
	if err != nil {
		return models.Clan{}, models.ClanMember{}, err
	}
	if !models.CanReviewClanApplications(member.Rank) {
		return models.Clan{}, models.ClanMember{}, ErrNotAllowed
	}
	return clan, member, nil
}
