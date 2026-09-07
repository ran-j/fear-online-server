package proudnet

import (
	"encoding/binary"
	"testing"
)

func TestRecordDeath(t *testing.T) {
	t.Run("a kill credits one side and costs the other", func(t *testing.T) {
		room := &Room{MaxUsers: 4}
		killer, _ := room.AddUser(RoomUser{Name: "A", Team: 1})
		victim, _ := room.AddUser(RoomUser{Name: "B", Team: 2})

		room.RecordDeath(killer.ID, victim.ID, 0)

		seats := room.Members()
		if seats[0].Score.Kills != 1 || seats[0].Score.Deaths != 0 {
			t.Fatalf("killer has %+v, want 1 kill and no deaths", seats[0].Score)
		}
		if seats[1].Score.Deaths != 1 || seats[1].Score.Kills != 0 {
			t.Fatalf("victim has %+v, want 1 death and no kills", seats[1].Score)
		}
	})

	t.Run("killing yourself only costs a death", func(t *testing.T) {
		room := &Room{MaxUsers: 4}
		user, _ := room.AddUser(RoomUser{Name: "A", Team: 1})

		room.RecordDeath(user.ID, user.ID, 0)

		if got := room.Members()[0].Score; got.Kills != 0 || got.Deaths != 1 {
			t.Fatalf("suicide scored %+v, want no kills and one death", got)
		}
	})
}

func TestFinishRound(t *testing.T) {
	t.Run("rounds advance until the limit", func(t *testing.T) {
		room := &Room{MaxUsers: 4, RoundLimit: 3}

		for want := uint8(1); want <= 2; want++ {
			played, last := room.FinishRound()
			if played != want || last {
				t.Fatalf("round %d reported played=%d last=%t", want, played, last)
			}
		}
		played, last := room.FinishRound()
		if played != 3 || !last {
			t.Fatalf("final round reported played=%d last=%t, want 3 and true", played, last)
		}
	})

	t.Run("a match with no limit never ends on time", func(t *testing.T) {
		room := &Room{MaxUsers: 4}

		if _, last := room.FinishRound(); last {
			t.Fatal("a room without a round limit ended after one round")
		}
	})
}

func TestUserResultRow(t *testing.T) {
	row := appendUserResult(nil, 1, RoomUser{Team: 2, Score: Score{Kills: 4, Deaths: 2, Assists: 3, Points: 900}})

	// seat + team + five counters + seven unmapped flags
	if want := 1 + 1 + 5*2 + 7; len(row) != want {
		t.Fatalf("Game::UserResult is %d bytes, want %d", len(row), want)
	}
	if row[0] != 1 || row[1] != 2 {
		t.Fatalf("seat/team are %d/%d, want 1/2", row[0], row[1])
	}
	// The counters do not go out in the order they are named: the client reads
	// them back as kill, death, assist, score from wire +6, +2, +4 and +8.
	for _, want := range []struct {
		at    int
		value uint16
		name  string
	}{{2, 2, "deaths"}, {4, 3, "assists"}, {6, 4, "kills"}, {8, 900, "points"}} {
		if got := binary.LittleEndian.Uint16(row[want.at:]); got != want.value {
			t.Fatalf("%s at wire +%d is %d, want %d", want.name, want.at, got, want.value)
		}
	}
}

func TestWinningTeam(t *testing.T) {
	cases := []struct {
		name  string
		users []RoomUser
		want  byte
	}{
		{"one side ahead", []RoomUser{{Team: 1, Score: Score{Kills: 3}}, {Team: 2, Score: Score{Kills: 1}}}, 1},
		{"the other side ahead", []RoomUser{{Team: 1, Score: Score{Kills: 1}}, {Team: 2, Score: Score{Kills: 5}}}, 2},
		{"a draw belongs to nobody", []RoomUser{{Team: 1, Score: Score{Kills: 2}}, {Team: 2, Score: Score{Kills: 2}}}, 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := winningTeam(test.users); got != test.want {
				t.Fatalf("winner is team %d, want %d", got, test.want)
			}
		})
	}
}

func TestRecordAssist(t *testing.T) {
	room := &Room{MaxUsers: 4}
	killer, _ := room.AddUser(RoomUser{Name: "A", Team: 1})
	victim, _ := room.AddUser(RoomUser{Name: "B", Team: 2})
	helper, _ := room.AddUser(RoomUser{Name: "C", Team: 1})

	room.RecordDeath(killer.ID, victim.ID, helper.ID)

	seats := room.Members()
	if seats[2].Score.Assists != 1 {
		t.Fatalf("helper has %+v, want one assist", seats[2].Score)
	}
	if seats[0].Score.Kills != 1 || seats[1].Score.Deaths != 1 {
		t.Fatalf("kill/death went astray: %+v %+v", seats[0].Score, seats[1].Score)
	}
}

// 0x9249 is the report minus its u32: the receiving side builds that field
// itself rather than reading it off the wire.
func TestNotifyDeathRow(t *testing.T) {
	report := []byte{7, 3, 2, 0x05, 0xbd, 0x6f, 0x01, 9, 8, 6}

	row := notifyDeathRow(report)

	want := []byte{7, 3, 2, 9, 8, 6}
	if len(row) != len(want) {
		t.Fatalf("NotifyDeath is %d bytes, want %d", len(row), len(want))
	}
	for i := range want {
		if row[i] != want[i] {
			t.Fatalf("byte %d is %d, want %d", i, row[i], want[i])
		}
	}
}
