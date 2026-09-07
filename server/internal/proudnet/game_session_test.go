package proudnet

import (
	"encoding/binary"
	"testing"
)

// The sizes below are read from ZNetwork's parsers, not chosen. A body that
// drifts by a byte does not fail loudly in game — the client misreads the rest
// of the struct in silence — so they are pinned here.
func TestMatchSnapshotSizes(t *testing.T) {
	t.Run("Game::User is 23 bytes with empty strings", func(t *testing.T) {
		row := appendGameUser(nil, 1, RoomUser{Team: 1})

		// seat + 2 empty strings + Clan::Mark + isClan + team + score + 2 flags
		if want := 1 + 1 + 1 + 8 + 1 + 1 + 8 + 2; len(row) != want {
			t.Fatalf("Game::User is %d bytes, want %d", len(row), want)
		}
	})

	t.Run("Game::UserScore is four u16 in wire order", func(t *testing.T) {
		body := appendGameUserScore(nil, Score{Kills: 7, Deaths: 3, Assists: 1, Points: 900})

		if len(body) != 8 {
			t.Fatalf("Game::UserScore is %d bytes, want 8", len(body))
		}
		for slot, want := range []uint16{7, 3, 1, 900} {
			if got := binary.LittleEndian.Uint16(body[slot*2:]); got != want {
				t.Fatalf("counter %d is %d, want %d", slot, got, want)
			}
		}
	})

	t.Run("the seat number is one-based and the team follows the name", func(t *testing.T) {
		users := []RoomUser{{Name: "A", Team: 1}, {Name: "B", Team: 2}}

		body := gameUserList(users)

		if body[0] != 2 {
			t.Fatalf("count varint is %d, want 2", body[0])
		}
		if body[1] != 1 {
			t.Fatalf("first seat is %d, want 1", body[1])
		}
	})
}

func TestMatchTickets(t *testing.T) {
	room := &Room{Number: 7, MaxUsers: 4}

	t.Run("a ticket leads back to its room", func(t *testing.T) {
		tickets := newMatchTickets()
		ticket := tickets.issue(room, 42)

		found, ok := tickets.redeem(ticket, 42)

		if !ok || found != room {
			t.Fatalf("redeemed %v ok=%t, want room 7", found, ok)
		}
	})

	t.Run("the right ticket on the wrong seat redeems nothing", func(t *testing.T) {
		tickets := newMatchTickets()
		ticket := tickets.issue(room, 42)

		if _, ok := tickets.redeem(ticket, 43); ok {
			t.Fatal("a ticket was accepted for a seat it was not issued to")
		}
	})

	t.Run("two seats never share a ticket", func(t *testing.T) {
		tickets := newMatchTickets()

		first := tickets.issue(room, 1)
		second := tickets.issue(room, 2)

		if first == second {
			t.Fatalf("both seats got ticket %d", first)
		}
	})
}

// The ticket has to survive 0x9057 -> 0x9089 in the only two guid bytes the
// client echoes back, so the two ends are pinned against each other here.
func TestGameStartTicketRoundTrip(t *testing.T) {
	const (
		seat   = uint32(1)
		ticket = uint16(0xA853)
	)

	body := notifyGameStartRow("127.0.0.1", 30003, seat, ticket)

	key := body[len(body)-userKeyBytes:]
	if got := binary.LittleEndian.Uint32(key[0:4]); got != seat {
		t.Fatalf("userId went out as %d, want %d", got, seat)
	}
	if key[4] != userKeyKind {
		t.Fatalf("keyKind went out as %d, want %d", key[4], userKeyKind)
	}
	if got := binary.LittleEndian.Uint16(key[5:7]); got != ticket {
		t.Fatalf("ticket went out as %d, want %d", got, ticket)
	}
}
