package models

import "time"

type Player struct {
	SteamID string `bson:"_id" json:"steamId"`
	Name    string `bson:"name" json:"name"`

	// Progression. Exp is the source of truth; Level is resolved from it via ClassInfo.csv
	Exp   uint64 `bson:"exp" json:"exp"`
	Level uint32 `bson:"level" json:"level"`

	// Persistent currencies (uint64 on the wire). Not the per-round economy.
	Point uint64 `bson:"point" json:"point"`
	Cash  uint64 `bson:"cash" json:"cash"`

	Inventory   []InventoryItem `bson:"inventory" json:"inventory"`
	Loadout     Loadout         `bson:"loadout" json:"loadout"`
	Attachments Attachments     `bson:"attachments" json:"attachments"`
	Records     Records         `bson:"records" json:"records"`
	ClanName    string          `bson:"clan_name,omitempty" json:"clanName,omitempty"`

	Friends        []Friend        `bson:"friends,omitempty" json:"friends,omitempty"`
	FriendRequests []FriendRequest `bson:"friend_requests,omitempty" json:"friendRequests,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"createdAt"`
	LastLogin time.Time `bson:"last_login" json:"lastLogin"`
}

func (p Player) Item(serial uint16) (InventoryItem, bool) {
	for _, item := range p.Inventory {
		if item.Serial == serial {
			return item, true
		}
	}
	return InventoryItem{}, false
}

func (p *Player) PurgeExpired(now time.Time) []uint16 {
	var removed []uint16
	kept := make([]InventoryItem, 0, len(p.Inventory))
	for _, item := range p.Inventory {
		if item.IsExpired(now) {
			removed = append(removed, item.Serial)
			continue
		}
		kept = append(kept, item)
	}
	if len(removed) == 0 {
		return nil
	}
	p.Inventory = kept

	gone := make(map[uint16]bool, len(removed))
	for _, serial := range removed {
		gone[serial] = true
	}
	attachments := make(Attachments, 0, len(p.Attachments))
	for _, mounted := range p.Attachments {
		if !gone[mounted.ChildSerial] && !gone[mounted.ParentSerial] {
			attachments = append(attachments, mounted)
		}
	}
	p.Attachments = attachments

	loadout := make(Loadout, 0, len(p.Loadout))
	for _, equipped := range p.Loadout {
		if !gone[equipped.Serial] {
			loadout = append(loadout, equipped)
		}
	}
	p.Loadout = loadout
	return removed
}

func (p Player) NextSerial() uint16 {
	used := make(map[uint16]bool, len(p.Inventory))
	for _, item := range p.Inventory {
		used[item.Serial] = true
	}
	for serial := uint16(1); serial < 0xFFFF; serial++ {
		if !used[serial] {
			return serial
		}
	}
	return 0
}

type Wallet uint8

const (
	WalletNone Wallet = iota
	WalletPoint
	WalletCash
)

func (p Player) Balance(w Wallet) uint64 {
	if w == WalletCash {
		return p.Cash
	}
	return p.Point
}

func (p *Player) Debit(w Wallet, amount uint64) {
	switch w {
	case WalletCash:
		p.Cash -= amount
	case WalletPoint:
		p.Point -= amount
	}
}
