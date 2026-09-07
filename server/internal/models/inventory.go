package models

import "time"

type InventoryItem struct {
	Serial      uint16    `bson:"serial" json:"serial"`
	ItemIndex   uint32    `bson:"item_index" json:"itemIndex"`
	Value       uint32    `bson:"value" json:"value"`
	State       uint8     `bson:"state" json:"state"`
	ItemType    uint8     `bson:"item_type" json:"itemType"`
	InstanceAux uint8     `bson:"instance_aux,omitempty" json:"instanceAux,omitempty"`
	ExpiresAt   time.Time `bson:"expires_at,omitempty" json:"expiresAt,omitempty"`
}

func (i InventoryItem) IsRental() bool { return !i.ExpiresAt.IsZero() }

func (i InventoryItem) IsExpired(now time.Time) bool {
	return i.IsRental() && !now.Before(i.ExpiresAt)
}

func (i InventoryItem) WireValue(now time.Time) uint32 {
	if !i.IsRental() {
		return i.Value
	}
	remaining := i.ExpiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	return uint32(remaining / time.Second)
}

type EquippedItem struct {
	Slot   uint8  `bson:"slot" json:"slot"`
	Serial uint16 `bson:"serial" json:"serial"`
}

// Loadout is the set of currently equipped items. The player equips
// one character per faction: an ATC character in SlotCharacterATC and a TF one in
// SlotCharacterTF. A character is an owned item whose faction is encoded in its
// ItemIndex (11xxxxxx = A.T.C., 12xxxxxx = T.F.).
type Loadout []EquippedItem

func (p *Player) ConsumeMaterials(need map[uint32]int) (deleted []uint16, updated []InventoryItem, ok bool) {
	remaining := make(map[uint32]int, len(need))
	for index, count := range need {
		remaining[index] = count
	}

	for index, count := range remaining {
		item, found := p.itemByIndex(index)
		if !found || int(item.Value) < count {
			return nil, nil, false
		}
	}

	next := make([]InventoryItem, 0, len(p.Inventory))
	for _, item := range p.Inventory {
		if count := remaining[item.ItemIndex]; count > 0 {
			remaining[item.ItemIndex] = 0 // take from this stack only
			if int(item.Value) <= count {
				deleted = append(deleted, item.Serial)
				continue
			}
			item.Value -= uint32(count)
			updated = append(updated, item)
		}
		next = append(next, item)
	}
	p.Inventory = next
	return deleted, updated, true
}

// SpentItem is the outcome of spending one unit of a stack: either the stack
// ran out and the item left the inventory, or it survived with one less unit.
type SpentItem struct {
	Serial    uint16
	UsedUp    bool
	Remaining InventoryItem
}

func (p *Player) SpendUnit(serial uint16) (SpentItem, bool) {
	for i := range p.Inventory {
		if p.Inventory[i].Serial != serial {
			continue
		}
		if p.Inventory[i].Value <= 1 {
			p.Inventory = append(p.Inventory[:i:i], p.Inventory[i+1:]...)
			return SpentItem{Serial: serial, UsedUp: true}, true
		}
		p.Inventory[i].Value--
		return SpentItem{Serial: serial, Remaining: p.Inventory[i]}, true
	}
	return SpentItem{}, false
}

func (p Player) itemByIndex(itemIndex uint32) (InventoryItem, bool) {
	for _, item := range p.Inventory {
		if item.ItemIndex == itemIndex {
			return item, true
		}
	}
	return InventoryItem{}, false
}

type AttachedItem struct {
	ParentSerial uint16 `bson:"parent_serial" json:"parentSerial"`
	ChildSerial  uint16 `bson:"child_serial" json:"childSerial"`
	Slot         uint8  `bson:"slot" json:"slot"`
}

type Attachments []AttachedItem

func (a Attachments) Attach(parent, child uint16, slot uint8) Attachments {
	next := make(Attachments, 0, len(a)+1)
	for _, mounted := range a {
		if mounted.ParentSerial != parent || mounted.Slot != slot {
			next = append(next, mounted)
		}
	}
	return append(next, AttachedItem{ParentSerial: parent, ChildSerial: child, Slot: slot})
}

func (a Attachments) Detach(parent, child uint16) Attachments {
	next := make(Attachments, 0, len(a))
	for _, mounted := range a {
		if mounted.ParentSerial != parent || mounted.ChildSerial != child {
			next = append(next, mounted)
		}
	}
	return next
}

// Equip places serial in slot, replacing whatever occupied it (one item per slot).
func (l Loadout) Equip(slot uint8, serial uint16) Loadout {
	next := make(Loadout, 0, len(l)+1)
	for _, equipped := range l {
		if equipped.Slot != slot {
			next = append(next, equipped)
		}
	}
	return append(next, EquippedItem{Slot: slot, Serial: serial})
}

func IsCustomSlot(slot uint8) bool {
	return slot >= SlotCustomPartFirst && slot <= SlotCustomPartLast
}

func (a Attachments) CustomParts() Attachments { return a.filter(true) }

func (a Attachments) GearAndPerks() Attachments { return a.filter(false) }

func (a Attachments) filter(custom bool) Attachments {
	out := make(Attachments, 0, len(a))
	for _, mounted := range a {
		if IsCustomSlot(mounted.Slot) == custom {
			out = append(out, mounted)
		}
	}
	return out
}

// Equip slots. The two hero slots are the per-faction character
// selection; the rest are the shared loadout.
const (
	SlotCharacterATC    uint8 = 0x0B
	SlotCharacterTF     uint8 = 0x0C
	SlotWeaponFirst     uint8 = 0x15
	SlotWeaponLast      uint8 = 0x19
	SlotWearFirst       uint8 = 0x1F
	SlotWearLast        uint8 = 0x22
	SlotCustomPartFirst uint8 = 0x3D
	SlotCustomPartLast  uint8 = 0x42
	SlotPerkFirst       uint8 = 0x51
	SlotPerkLast        uint8 = 0x53
)
