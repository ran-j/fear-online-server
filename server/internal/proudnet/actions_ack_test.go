package proudnet

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestMenuResponses(t *testing.T) {
	server := NewServer(nil, nil, "", 0)
	NewGameActionHandle(server)
	for _, menu := range []struct {
		request, answer uint16
		destination     byte
	}{
		{0x9859, 0x98BD, 0}, // inventory / store open
		{0x985A, 0x98BF, 4}, // inventory / store close
		{0x99E9, 0x9A4D, 0}, // crafting open
		{0x99EA, 0x9A4F, 0}, // crafting open new
		{0x99EB, 0x9A51, 4}, // crafting close
		{0x9AB1, 0x9B15, 0}, // perks open
		{0x9AB2, 0x9B17, 0}, // perks open new
		{0x9AB3, 0x9B19, 4}, // perks close
	} {
		for _, inRoom := range []bool{false, true} {
			t.Run(fmt.Sprintf("%04x/room=%t", menu.request, inRoom), func(t *testing.T) {
				client, conn := net.Pipe()
				defer client.Close()
				defer conn.Close()
				client.SetDeadline(time.Now().Add(time.Second))
				conn.SetDeadline(time.Now().Add(time.Second))
				session := &Session{conn: conn, State: &sessionData{}}
				destination := menu.destination
				if inRoom {
					setSessionRoom(session, &Room{Number: 1})
					if destination != 0 {
						destination = 5
					}
				}
				done := make(chan error, 1)
				go func() {
					done <- server.handlers[menu.request](session, Message{ID: menu.request})
					conn.Close()
				}()
				got, err := io.ReadAll(client)
				if err != nil {
					t.Fatal(err)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
				// Only close replies append the u8 destination to the plain RMI.
				want := []byte{0x13, 0x57, 3, 1, byte(menu.answer), byte(menu.answer >> 8)}
				if destination != 0 {
					want[2] = 4
					want = append(want, destination)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("menu reply = %x, want %x", got, want)
				}
			})
		}
	}
}
