package models

import "time"

type Clan struct {
	Name      string   `bson:"_id" json:"name"`
	Mark      [3]int32 `bson:"mark" json:"mark"`
	MarkExtra uint16   `bson:"mark_extra,omitempty" json:"markExtra,omitempty"`

	Level          uint8 `bson:"level,omitempty" json:"level,omitempty"`
	MemberCapacity uint8 `bson:"member_capacity,omitempty" json:"memberCapacity,omitempty"`

	Master     string          `bson:"master" json:"master"` // master SteamID
	Members    []ClanMember    `bson:"members" json:"members"`
	Applicants []ClanApplicant `bson:"applicants,omitempty" json:"applicants,omitempty"`
	Records    ClanRecord      `bson:"records" json:"records"`
	CreatedAt  time.Time       `bson:"created_at" json:"createdAt"`
}

type ClanApplicant struct {
	ApplicantID uint16 `bson:"applicant_id" json:"applicantId"`

	SteamID   string    `bson:"steam_id" json:"steamId"`
	Name      string    `bson:"name" json:"name"`
	Level     uint32    `bson:"level" json:"level"`
	Message   string    `bson:"message" json:"message"`
	AppliedAt time.Time `bson:"applied_at" json:"appliedAt"`
}

type ClanMember struct {
	MemberID uint16     `bson:"member_id" json:"memberId"`
	SteamID  string     `bson:"steam_id" json:"steamId"`
	Name     string     `bson:"name" json:"name"`
	Rank     uint8      `bson:"rank" json:"rank"`
	Records  ClanRecord `bson:"records" json:"records"`
}

const (
	ClanRankMaster    uint8 = 1
	ClanRankManager   uint8 = 2
	ClanRankMember    uint8 = 8
	ClanRankAssociate uint8 = 9

	DefaultClanMemberCapacity uint8 = 50
)

func CanReviewClanApplications(rank uint8) bool {
	return rank == ClanRankMaster || rank == ClanRankManager
}

func IsAssignableClanRank(rank uint8) bool {
	return rank == ClanRankManager || rank == ClanRankMember || rank == ClanRankAssociate
}

func (c Clan) Member(steamID string) (ClanMember, bool) {
	for _, member := range c.Members {
		if member.SteamID == steamID {
			return member, true
		}
	}
	return ClanMember{}, false
}

func (c Clan) MemberByID(memberID uint16) (ClanMember, bool) {
	for _, member := range c.Members {
		if member.MemberID == memberID {
			return member, true
		}
	}
	return ClanMember{}, false
}

func (c Clan) Applicant(applicantID uint16) (ClanApplicant, bool) {
	for _, applicant := range c.Applicants {
		if applicant.ApplicantID == applicantID {
			return applicant, true
		}
	}
	return ClanApplicant{}, false
}

func (c Clan) Capacity() uint8 {
	if c.MemberCapacity == 0 {
		return DefaultClanMemberCapacity
	}
	return c.MemberCapacity
}

func (c Clan) IsFull() bool {
	return len(c.Members) >= int(c.Capacity())
}

func (c *Clan) RemoveMember(memberID uint16) (ClanMember, bool) {
	for i, member := range c.Members {
		if member.MemberID == memberID {
			c.Members = append(c.Members[:i:i], c.Members[i+1:]...)
			return member, true
		}
	}
	return ClanMember{}, false
}

func (c *Clan) RemoveApplicant(applicantID uint16) (ClanApplicant, bool) {
	for i, applicant := range c.Applicants {
		if applicant.ApplicantID == applicantID {
			c.Applicants = append(c.Applicants[:i:i], c.Applicants[i+1:]...)
			return applicant, true
		}
	}
	return ClanApplicant{}, false
}

func (c *Clan) SetRank(memberID uint16, rank uint8) bool {
	for i := range c.Members {
		if c.Members[i].MemberID == memberID {
			c.Members[i].Rank = rank
			return true
		}
	}
	return false
}

type ClanRecord struct {
	Win   int32 `bson:"win" json:"win"`
	Draw  int32 `bson:"draw" json:"draw"`
	Lose  int32 `bson:"lose" json:"lose"`
	Kill  int32 `bson:"kill" json:"kill"`
	Death int32 `bson:"death" json:"death"`
}
