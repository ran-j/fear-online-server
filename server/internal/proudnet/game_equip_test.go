package proudnet

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"go-service-template/internal/models"
	"go-service-template/internal/repositories"
	"go-service-template/internal/services"
	"go-service-template/pkg/logger"

	"github.com/stretchr/testify/mock"
)

func TestWeaponChangePreservesMatchAndNotifiesHost(t *testing.T) {
	repo := repositories.NewMemoryPlayerRepository()
	player := models.Player{SteamID: "weapon-test", Inventory: []models.InventoryItem{
		{Serial: 4, ItemIndex: 21605602, ItemType: models.SlotWeaponFirst},
		{Serial: 9, ItemIndex: 21100101, ItemType: models.SlotWeaponFirst},
	}, Loadout: models.Loadout{{Slot: models.SlotWeaponFirst, Serial: 4}}}
	if err := repo.Create(context.Background(), player); err != nil {
		t.Fatal(err)
	}
	log := &logger.Mock{}
	log.On("Info", mock.Anything).Return()
	lobby := NewLobby(log, nil, services.NewPlayerService(repo, nil, 0, 0), nil)
	room := &Room{Number: 7, Users: []RoomUser{{ID: 3}, {ID: 8, SteamID: player.SteamID}}}
	client, conn := net.Pipe()
	defer client.Close()
	defer conn.Close()
	client.SetDeadline(time.Now().Add(2 * time.Second))
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	session := &Session{conn: conn}
	bindMatchSession(session, player, room, 8)
	setSessionChannel(session, 3)
	lobby.matches.join(room.Number, session)
	body := make([]byte, changeWeaponBytes)
	body[0] = 2 // match seat, not the room's persistent ID
	binary.LittleEndian.PutUint16(body[1:], 9)
	done := make(chan error, 1)
	go func() {
		err := lobby.changeWeapon(session, Message{ID: rmiRequestChangeWeapon, Body: body})
		if err == nil {
			err = lobby.respawn(session, Message{})
		}
		done <- err
		conn.Close()
	}()
	got, err := io.ReadAll(client)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	want := []byte{0x13, 0x57, 4, 1, 0x5b, 0x92, 2}
	want = append(want, 0x13, 0x57, 48, 1, 0x5d, 0x92, 2)
	want = appendUint32(want, 21100101)
	want = append(want, make([]byte, 40)...)
	want = append(want, 0x13, 0x57, 4, 1, 0x42, 0x92, 2)
	want = append(want, 0x13, 0x57, 4, 1, 0x44, 0x92, 2)
	if !bytes.Equal(got, want) {
		t.Fatalf("weapon change then respawn = %x, want %x", got, want)
	}
	if sessionRoom(session) != room || sessionRoomUser(session) != 8 || sessionChannel(session) != 3 {
		t.Fatal("weapon change lost the match context")
	}
	updated, _ := resolvePlayer(session)
	if got := buildUserEquip(2, updated).Weapon[0]; got != 21100101 {
		t.Fatalf("session primary weapon = %d", got)
	}
}

func TestMatchSnapshotIncludesLocalTeam(t *testing.T) {
	repo := repositories.NewMemoryPlayerRepository()
	lobby := NewLobby(nil, nil, services.NewPlayerService(repo, nil, 0, 0), nil)
	users := []RoomUser{{ID: 3, Team: 1}, {ID: 8, Team: 2}}
	room := &Room{Number: 7, LeaderID: 3, Users: users}
	for _, user := range users {
		found := false
		for _, msg := range lobby.matchSnapshot(room, users, user.ID) {
			if msg.ID == rmiNotifyGameInfo {
				found = true
				want := []byte{seatIndex(users, user.ID), 1, user.Team, user.Team}
				if !bytes.Equal(msg.Body, want) {
					t.Fatalf("NotifyInfo = %x, want %x", msg.Body, want)
				}
			}
		}
		if !found {
			t.Fatal("snapshot omitted NotifyInfo")
		}
	}
}
