package proudnet

import (
	"bytes"
	"testing"

	"go-service-template/internal/models"
)

func TestAppendClanInfoCarriesLevelAndMemberCounts(t *testing.T) {
	clan := models.Clan{
		Name:           "A",
		Master:         "steam-master",
		Level:          3,
		MemberCapacity: 10,
		Members: []models.ClanMember{
			{SteamID: "steam-master", Name: "M", Rank: models.ClanRankMaster},
			{SteamID: "steam-manager", Name: "G", Rank: models.ClanRankManager},
		},
	}

	got := appendClanInfo(nil, clan)

	// u32 id + "A" + "M" puts the two one-byte header fields at offsets
	// 10 and 11. The second byte is the clan level.
	if got[11] != 3 {
		t.Fatalf("clan level byte = %d, want 3", got[11])
	}

	// After the level come u64 experience, six u32 record fields, one rank
	// byte, the eight-byte mark and one unknown presentation byte.
	wantCounts := []byte{2, 1, 0, 0, 10}
	if !bytes.Equal(got[54:59], wantCounts) {
		t.Fatalf("embedded member counts = %x, want %x", got[54:59], wantCounts)
	}
}
