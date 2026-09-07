package proudnet

import "testing"

// Two clients on the same server must never look like the same person, which is
// what a constant character id did.
func TestCharacterID(t *testing.T) {
	t.Run("a SteamID64 yields its account number", func(t *testing.T) {
		// 76561198114269084 - 76561197960265728
		if got := characterID("76561198114269084"); got != 154003356 {
			t.Fatalf("account number is %d, want 154003356", got)
		}
	})

	t.Run("different accounts never share an id", func(t *testing.T) {
		first := characterID("76561198114269084")
		second := characterID("76561198114269085")

		if first == second {
			t.Fatalf("two accounts both got id %d", first)
		}
	})

	t.Run("the same account is stable across calls", func(t *testing.T) {
		if characterID("76561198114269084") != characterID("76561198114269084") {
			t.Fatal("the same account got two different ids")
		}
	})

	t.Run("an id that is not a SteamID64 still gets something distinct", func(t *testing.T) {
		bot := characterID("test-account-a")
		other := characterID("test-account-b")

		if bot == 0 || bot == other {
			t.Fatalf("non-steam accounts collided or vanished: %d vs %d", bot, other)
		}
	})
}
