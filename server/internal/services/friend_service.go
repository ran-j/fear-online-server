package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"go-service-template/internal/models"
	"go-service-template/internal/repositories"
)

var (
	ErrFriendNotFound    = errors.New("player not found by name")
	ErrAlreadyFriends    = errors.New("already friends")
	ErrFriendSelf        = errors.New("cannot befriend yourself")
	ErrRequestPending    = errors.New("a request is already pending")
	ErrRequestNotFound   = errors.New("friend request not found")
	ErrFriendListMissing = errors.New("friend not on the list")
)

type FriendService struct {
	players repositories.PlayerRepository
}

func NewFriendService(players repositories.PlayerRepository) *FriendService {
	return &FriendService{players: players}
}

func (s *FriendService) actor(ctx context.Context, steamID string) (models.Player, error) {
	player, found, err := s.players.Find(ctx, steamID)
	if err != nil {
		return models.Player{}, err
	}
	if !found {
		return models.Player{}, ErrPlayerNotFound
	}
	return player, nil
}

func (s *FriendService) Request(ctx context.Context, senderID, targetName string) (models.Player, models.FriendRequest, error) {
	sender, err := s.actor(ctx, senderID)
	if err != nil {
		return models.Player{}, models.FriendRequest{}, err
	}

	target, found, err := s.players.FindByName(ctx, strings.TrimSpace(targetName))
	if err != nil {
		return models.Player{}, models.FriendRequest{}, err
	}
	if !found {
		return models.Player{}, models.FriendRequest{}, ErrFriendNotFound
	}
	if target.SteamID == sender.SteamID {
		return models.Player{}, models.FriendRequest{}, ErrFriendSelf
	}
	if target.IsFriendOf(sender.SteamID) {
		return models.Player{}, models.FriendRequest{}, ErrAlreadyFriends
	}
	if existing, ok := target.FriendRequestFrom(sender.SteamID); ok {
		return target, existing, ErrRequestPending
	}

	request := models.FriendRequest{
		RequestID: target.NextSocialID(),
		SteamID:   sender.SteamID,
		SentAt:    time.Now().UTC(),
	}
	target.FriendRequests = append(target.FriendRequests, request)
	if err := s.players.SaveSocial(ctx, target.SteamID, target.Friends, target.FriendRequests); err != nil {
		return models.Player{}, models.FriendRequest{}, err
	}
	return target, request, nil
}

func (s *FriendService) Accept(ctx context.Context, actorID string, requestID uint16) (models.Friend, models.Friend, models.Player, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return models.Friend{}, models.Friend{}, models.Player{}, err
	}

	request, ok := actor.RemoveFriendRequest(requestID)
	if !ok {
		return models.Friend{}, models.Friend{}, models.Player{}, ErrRequestNotFound
	}

	sender, found, err := s.players.Find(ctx, request.SteamID)
	if err != nil {
		return models.Friend{}, models.Friend{}, models.Player{}, err
	}
	if !found {
		return models.Friend{}, models.Friend{}, models.Player{},
			s.players.SaveSocial(ctx, actor.SteamID, actor.Friends, actor.FriendRequests)
	}

	now := time.Now().UTC()
	mine := models.Friend{FriendID: requestID, SteamID: sender.SteamID, Since: now}
	theirs := models.Friend{FriendID: sender.NextSocialID(), SteamID: actor.SteamID, Since: now}

	actor.Friends = append(actor.Friends, mine)
	sender.Friends = append(sender.Friends, theirs)

	if err := s.players.SaveSocial(ctx, actor.SteamID, actor.Friends, actor.FriendRequests); err != nil {
		return models.Friend{}, models.Friend{}, models.Player{}, err
	}
	if err := s.players.SaveSocial(ctx, sender.SteamID, sender.Friends, sender.FriendRequests); err != nil {
		return models.Friend{}, models.Friend{}, models.Player{}, err
	}
	return mine, theirs, sender, nil
}

func (s *FriendService) Reject(ctx context.Context, actorID string, requestID uint16) (models.FriendRequest, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return models.FriendRequest{}, err
	}

	request, ok := actor.RemoveFriendRequest(requestID)
	if !ok {
		return models.FriendRequest{}, ErrRequestNotFound
	}
	if err := s.players.SaveSocial(ctx, actor.SteamID, actor.Friends, actor.FriendRequests); err != nil {
		return models.FriendRequest{}, err
	}
	return request, nil
}

func (s *FriendService) Delete(ctx context.Context, actorID string, friendID uint16) (models.Friend, models.Friend, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return models.Friend{}, models.Friend{}, err
	}

	friend, ok := actor.RemoveFriend(friendID)
	if !ok {
		return models.Friend{}, models.Friend{}, ErrFriendListMissing
	}
	if err := s.players.SaveSocial(ctx, actor.SteamID, actor.Friends, actor.FriendRequests); err != nil {
		return models.Friend{}, models.Friend{}, err
	}

	other, found, err := s.players.Find(ctx, friend.SteamID)
	if err != nil {
		return models.Friend{}, models.Friend{}, err
	}
	if !found {
		return friend, models.Friend{}, nil
	}
	mirrored, ok := other.RemoveFriendOf(actor.SteamID)
	if !ok {
		return friend, models.Friend{}, nil
	}
	if err := s.players.SaveSocial(ctx, other.SteamID, other.Friends, other.FriendRequests); err != nil {
		return models.Friend{}, models.Friend{}, err
	}
	return friend, mirrored, nil
}

// Lists loads a player's current social state plus every account it references,
// which is what the panel needs to draw itself.
func (s *FriendService) Lists(ctx context.Context, steamID string) (models.Player, map[string]models.Player, error) {
	player, err := s.actor(ctx, steamID)
	if err != nil {
		return models.Player{}, nil, err
	}
	accounts, err := s.accounts(ctx, player)
	return player, accounts, err
}

func (s *FriendService) accounts(ctx context.Context, player models.Player) (map[string]models.Player, error) {
	steamIDs := make([]string, 0, len(player.Friends)+len(player.FriendRequests))
	for _, friend := range player.Friends {
		steamIDs = append(steamIDs, friend.SteamID)
	}
	for _, request := range player.FriendRequests {
		steamIDs = append(steamIDs, request.SteamID)
	}
	if len(steamIDs) == 0 {
		return nil, nil
	}
	return s.players.FindMany(ctx, steamIDs)
}

func (s *FriendService) Find(ctx context.Context, steamID string) (models.Player, bool, error) {
	return s.players.Find(ctx, steamID)
}

// InviteTarget resolves a friend for an invitation: the account being invited
// plus the id the inviter is known by on that account's list, which is how the
// invited client tells who is calling.
func (s *FriendService) InviteTarget(ctx context.Context, actorID string, friendID uint16) (models.Player, uint16, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return models.Player{}, 0, err
	}

	friend, ok := actor.Friend(friendID)
	if !ok {
		return models.Player{}, 0, ErrFriendListMissing
	}
	target, found, err := s.players.Find(ctx, friend.SteamID)
	if err != nil {
		return models.Player{}, 0, err
	}
	if !found {
		return models.Player{}, 0, ErrPlayerNotFound
	}

	mirrored, ok := target.FriendEntryFor(actorID)
	if !ok {
		return models.Player{}, 0, ErrFriendListMissing
	}
	return target, mirrored.FriendID, nil
}
