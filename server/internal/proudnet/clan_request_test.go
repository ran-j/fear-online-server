package proudnet

import (
	"bytes"
	"testing"

	"go-service-template/internal/models"
)

func TestAppendRequestClanInfoPacksMarkAsymmetrically(t *testing.T) {
	clan := models.Clan{Name: "ABC", Mark: [3]int32{0x1234, 0x56, 0x78}}

	got := appendRequestClanInfo(nil, clan)

	want := []byte{
		0x03,
		'A', 0, 'B', 0, 'C', 0,
	}
	want = appendUint32(want, clanWireID(clan.Name))
	// mark0 keeps 16 bits, mark1 and mark2 one byte each.
	want = appendUint32(want, 0x78561234)

	if !bytes.Equal(got, want) {
		t.Fatalf("SendRequestClanInfo body = %x, want %x", got, want)
	}
}

func TestPackClanMarkClampsOversizedComponents(t *testing.T) {
	clan := models.Clan{Mark: [3]int32{0x1FFFF, 0x1FF, -1}}

	if got := packClanMark(clan); got != 0x00FFFFFF {
		t.Fatalf("packClanMark = %08x, want 00ffffff", got)
	}
}
