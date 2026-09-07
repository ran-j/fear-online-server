package proudnet

import (
	"sync"
	"testing"
)

func TestRoomRegistry(t *testing.T) {
	t.Run("hands out distinct numbers and finds them back", func(t *testing.T) {
		registry := NewRoomRegistry()

		first := registry.Open(&Room{Title: "one", MaxUsers: 4})
		second := registry.Open(&Room{Title: "two", MaxUsers: 4})

		if first == second {
			t.Fatalf("two rooms share number %d", first)
		}
		room, ok := registry.Find(second)
		if !ok || room.Title != "two" {
			t.Fatalf("room %d did not come back", second)
		}
	})

	t.Run("a closed room stops resolving", func(t *testing.T) {
		registry := NewRoomRegistry()
		number := registry.Open(&Room{Title: "gone", MaxUsers: 4})

		registry.Close(number)

		if _, ok := registry.Find(number); ok {
			t.Fatal("closed room still resolves")
		}
	})

	t.Run("listing is ordered by number", func(t *testing.T) {
		registry := NewRoomRegistry()
		for i := 0; i < 5; i++ {
			registry.Open(&Room{MaxUsers: 4})
		}

		rooms := registry.List()

		for i := 1; i < len(rooms); i++ {
			if rooms[i-1].Number >= rooms[i].Number {
				t.Fatalf("listing out of order at %d", i)
			}
		}
	})

	t.Run("concurrent opens never collide", func(t *testing.T) {
		registry := NewRoomRegistry()
		const count = 200

		var wait sync.WaitGroup
		numbers := make([]uint16, count)
		for i := 0; i < count; i++ {
			wait.Add(1)
			go func(slot int) {
				defer wait.Done()
				numbers[slot] = registry.Open(&Room{MaxUsers: 4})
			}(i)
		}
		wait.Wait()

		seen := make(map[uint16]bool, count)
		for _, number := range numbers {
			if seen[number] {
				t.Fatalf("number %d handed out twice", number)
			}
			seen[number] = true
		}
	})
}

func TestRoomMembers(t *testing.T) {
	t.Run("ids are unique and seating respects the cap", func(t *testing.T) {
		room := &Room{MaxUsers: 2}

		first, ok := room.AddUser(RoomUser{Name: "A", Team: 1})
		if !ok {
			t.Fatal("first seat refused")
		}
		second, ok := room.AddUser(RoomUser{Name: "B", Team: 2})
		if !ok {
			t.Fatal("second seat refused")
		}
		if first.ID == second.ID {
			t.Fatalf("both players got id %d", first.ID)
		}
		if _, ok := room.AddUser(RoomUser{Name: "C", Team: 1}); ok {
			t.Fatal("a full room accepted a third player")
		}
	})

	t.Run("removing the last member reports the room empty", func(t *testing.T) {
		room := &Room{MaxUsers: 4}
		user, _ := room.AddUser(RoomUser{Name: "A", Team: 1})

		_, found, empty := room.RemoveUser(user.ID)

		if !found || !empty {
			t.Fatalf("found=%t empty=%t, want both true", found, empty)
		}
	})

	t.Run("concurrent seating keeps every id distinct", func(t *testing.T) {
		room := &Room{MaxUsers: 100}

		var wait sync.WaitGroup
		for i := 0; i < 100; i++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				room.AddUser(RoomUser{Name: "player", Team: 1})
			}()
		}
		wait.Wait()

		seen := make(map[uint32]bool)
		for _, user := range room.Members() {
			if seen[user.ID] {
				t.Fatalf("id %d handed out twice", user.ID)
			}
			seen[user.ID] = true
		}
		if len(seen) != 100 {
			t.Fatalf("seated %d players, want 100", len(seen))
		}
	})
}
