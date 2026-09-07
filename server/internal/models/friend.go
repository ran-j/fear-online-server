package models

import "time"

type Friend struct {
	FriendID uint16    `bson:"friend_id" json:"friendId"`
	SteamID  string    `bson:"steam_id" json:"steamId"`
	Since    time.Time `bson:"since" json:"since"`
}
type FriendRequest struct {
	RequestID uint16    `bson:"request_id" json:"requestId"`
	SteamID   string    `bson:"steam_id" json:"steamId"`
	SentAt    time.Time `bson:"sent_at" json:"sentAt"`
}

func (p Player) Friend(friendID uint16) (Friend, bool) {
	for _, friend := range p.Friends {
		if friend.FriendID == friendID {
			return friend, true
		}
	}
	return Friend{}, false
}

func (p Player) IsFriendOf(steamID string) bool {
	for _, friend := range p.Friends {
		if friend.SteamID == steamID {
			return true
		}
	}
	return false
}

func (p Player) FriendRequestFrom(steamID string) (FriendRequest, bool) {
	for _, request := range p.FriendRequests {
		if request.SteamID == steamID {
			return request, true
		}
	}
	return FriendRequest{}, false
}

func (p *Player) RemoveFriend(friendID uint16) (Friend, bool) {
	for i, friend := range p.Friends {
		if friend.FriendID == friendID {
			p.Friends = append(p.Friends[:i:i], p.Friends[i+1:]...)
			return friend, true
		}
	}
	return Friend{}, false
}

func (p *Player) RemoveFriendOf(steamID string) (Friend, bool) {
	for i, friend := range p.Friends {
		if friend.SteamID == steamID {
			p.Friends = append(p.Friends[:i:i], p.Friends[i+1:]...)
			return friend, true
		}
	}
	return Friend{}, false
}

func (p *Player) RemoveFriendRequest(requestID uint16) (FriendRequest, bool) {
	for i, request := range p.FriendRequests {
		if request.RequestID == requestID {
			p.FriendRequests = append(p.FriendRequests[:i:i], p.FriendRequests[i+1:]...)
			return request, true
		}
	}
	return FriendRequest{}, false
}

func (p Player) NextSocialID() uint16 {
	highest := uint16(0)
	for _, friend := range p.Friends {
		if friend.FriendID > highest {
			highest = friend.FriendID
		}
	}
	for _, request := range p.FriendRequests {
		if request.RequestID > highest {
			highest = request.RequestID
		}
	}
	return highest + 1
}

func (p Player) FriendEntryFor(steamID string) (Friend, bool) {
	for _, friend := range p.Friends {
		if friend.SteamID == steamID {
			return friend, true
		}
	}
	return Friend{}, false
}
