package proudnet

import (
	"bytes"
	"testing"

	"go-service-template/internal/models"
)

func TestAppendClanMember(t *testing.T) {
	member := clanMemberWire{
		MemberID: 0x1234,
		Name:     "AB",
		Level:    7,
		PvP: clanMemberRecordWire{
			Win: 1, Lose: 2, Draw: 3, Kill: 4, Death: 5, Extra: 6,
		},
		ClanRecord: clanMemberRecordWire{
			Win: 11, Lose: 12, Draw: 13, Kill: 14, Death: 15, Extra: 16,
		},
		Channel:  2,
		Lobby:    3,
		Room:     0x4567,
		Position: 8,
		Online:   true,
		InGame:   false,
	}

	got := appendClanMember(nil, member)
	want := []byte{
		0x34, 0x12,
		0x02, 'A', 0x00, 'B', 0x00,
		0x07,
	}
	for value := uint32(1); value <= 6; value++ {
		want = appendUint32(want, value)
	}
	for value := uint32(11); value <= 16; value++ {
		want = appendUint32(want, value)
	}
	want = append(want,
		0x02,
		0x03,
		0x67, 0x45,
		0x08,
		0x01,
		0x00,
	)

	if !bytes.Equal(got, want) {
		t.Fatalf("appendClanMember mismatch\n got: %x\nwant: %x", got, want)
	}
}

func TestClanMemberList(t *testing.T) {
	member := clanMemberWire{MemberID: 1, Name: "X", Level: 1, Position: 9}
	got := clanMemberList([]clanMemberWire{member})
	want := append([]byte{0x01}, appendClanMember(nil, member)...)
	if !bytes.Equal(got, want) {
		t.Fatalf("clanMemberList mismatch\n got: %x\nwant: %x", got, want)
	}
}

func TestMakeClanMemberCounts(t *testing.T) {
	clan := models.Clan{
		MemberCapacity: 12,
		Members: []models.ClanMember{
			{Rank: models.ClanRankMaster},
			{Rank: models.ClanRankManager},
			{Rank: models.ClanRankMember},
			{Rank: models.ClanRankAssociate},
			{Rank: 0x7F},
		},
	}

	got := makeClanMemberCounts(clan)
	want := clanMemberCounts{
		CurrentTotal: 5,
		Managers:     1,
		Members:      1,
		Associates:   1,
		Capacity:     12,
	}
	if got != want {
		t.Fatalf("makeClanMemberCounts mismatch: got %+v want %+v", got, want)
	}

	wantRow := []byte{5, 1, 1, 1, 12}
	if !bytes.Equal(got.row(), wantRow) {
		t.Fatalf("member count row mismatch: got %x want %x", got.row(), wantRow)
	}
}

func TestMakeClanMemberCountsUsesCompatibleDefaults(t *testing.T) {
	members := make([]models.ClanMember, 51)
	got := makeClanMemberCounts(models.Clan{Members: members})
	if got.CurrentTotal != 51 {
		t.Fatalf("current total = %d, want 51", got.CurrentTotal)
	}
	if got.Capacity != 51 {
		t.Fatalf("capacity = %d, want at least the persisted member count", got.Capacity)
	}
}
