package proudnet

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"go-service-template/internal/models"
	"go-service-template/internal/services"
	"go-service-template/pkg/logger"
)

const (
	rmiRequestEquipItem uint16 = 0x985C
	rmiSendEquipSlot    uint16 = 0x7700
	rmiAnswerEquipItem  uint16 = 0x98C3

	rmiRequestBuyItem    uint16 = 0x985B
	rmiAnswerBuyItem     uint16 = 0x98C1
	rmiAnswerBuyItemFail uint16 = 0x98C2
	rmiSendItem          uint16 = 0x76F8
	rmiSendItemValue     uint16 = 0x76FE
	rmiSendDeleteItem    uint16 = 0x76FA

	rmiRequestRecipeItem    uint16 = 0x99EC
	rmiAnswerRecipeItem     uint16 = 0x9A53
	rmiAnswerRecipeItemFail uint16 = 0x9A54

	rmiRequestBuyCustomItem    uint16 = 0x9860
	rmiAnswerBuyCustomItem     uint16 = 0x98CB
	rmiAnswerBuyCustomItemFail uint16 = 0x98CC

	rmiRequestEquipCustomItem uint16 = 0x9861
	rmiAnswerEquipCustomItem  uint16 = 0x98CD
	rmiAnswerEquipCustomFail  uint16 = 0x98CE

	rmiRequestBreakCustomItem uint16 = 0x9862
	rmiAnswerBreakCustomItem  uint16 = 0x98CF
	rmiAnswerBreakCustomFail  uint16 = 0x98D0

	rmiRequestEquipGearItem uint16 = 0x9863
	rmiAnswerEquipGearItem  uint16 = 0x98D1
	rmiAnswerEquipGearFail  uint16 = 0x98D2

	rmiRequestBreakGearItem uint16 = 0x9864
	rmiAnswerBreakGearItem  uint16 = 0x98D3
	rmiAnswerBreakGearFail  uint16 = 0x98D4

	rmiRequestEquipPerkItem uint16 = 0x9AB4 // mount a perk (itemType 0x51-0x53) on a character
	rmiAnswerEquipPerkItem  uint16 = 0x9B1B
	rmiAnswerEquipPerkFail  uint16 = 0x9B1C
	rmiRequestBreakPerkItem uint16 = 0x9AB5
	rmiAnswerBreakPerkItem  uint16 = 0x9B1D
	rmiAnswerBreakPerkFail  uint16 = 0x9B1E
	rmiRequestIncPerkSlot   uint16 = 0x9AB6
	rmiAnswerIncPerkSlot    uint16 = 0x9B1F

	// Three different mount structures, with confusingly similar names:
	//   Item::Slot      (0x76FB/0x76FC) gear + perks   — 10 bytes
	//   Item::EquipSlot (0x76FF/0x7700) weapon loadout — 7 bytes
	//   Item::Equip     (0x7701/0x7702) weapon custom parts — 6 bytes
	rmiSendSlotList  uint16 = 0x76FB
	rmiSendSlot      uint16 = 0x76FC
	rmiSendEquipList uint16 = 0x7701
	rmiSendEquip     uint16 = 0x7702

	kindGear   = "gear"
	kindCustom = "custom"
	kindPerk   = "perk"

	// The trailing byte of a slot row: whether the mounted item is in use.
	slotInactive byte = 0
	slotActive   byte = 1
)

type Items struct {
	players *services.PlayerService
	logger  logger.Interface
}

func NewItems(players *services.PlayerService, logger logger.Interface) *Items {
	return &Items{players: players, logger: logger}
}

func (i *Items) Register(server *Server) {
	server.Handle(rmiRequestEquipItem, i.equip)
	server.Handle(rmiRequestBuyItem, i.buy)
	server.Handle(rmiRequestRecipeItem, i.craft)
	server.Handle(rmiRequestBuyCustomItem, i.buyCustom)
	server.Handle(rmiRequestEquipCustomItem, i.equipCustom)
	server.Handle(rmiRequestBreakCustomItem, i.breakCustom)
	server.Handle(rmiRequestEquipGearItem, i.gearEquip)
	server.Handle(rmiRequestBreakGearItem, i.gearBreak)
	server.Handle(rmiRequestEquipPerkItem, i.perkEquip)
	server.Handle(rmiRequestBreakPerkItem, i.perkBreak)
	server.Handle(rmiRequestIncPerkSlot, i.perkIncreaseSlot)
}

func (i *Items) perkEquip(session *Session, msg Message) error {
	i.logger.Info(fmt.Sprintf("perk: RequestEquipPerkItem body=%x len=%d", msg.Body, len(msg.Body)))
	if len(msg.Body) == 2 {
		serial := binary.LittleEndian.Uint16(msg.Body[:2])
		if err := i.equipSerial(session, serial, rmiAnswerEquipPerkItem); err != nil {
			i.logger.Info(fmt.Sprintf("perk: equip serial=%d rejected: %v", serial, err))
			return session.SendPlain(Message{ID: rmiAnswerEquipPerkFail, Body: int32LE(0)})
		}
		return nil
	}
	return i.attach(session, msg, kindPerk, rmiAnswerEquipPerkItem, rmiAnswerEquipPerkFail)
}

func (i *Items) perkBreak(session *Session, msg Message) error {
	i.logger.Info(fmt.Sprintf("perk: RequestBreakPerkItem body=%x len=%d", msg.Body, len(msg.Body)))
	return i.detach(session, msg, kindPerk, rmiAnswerBreakPerkItem, rmiAnswerBreakPerkFail)
}

// TODO: this should cost currency and the unlocked count should persist on the
// account; for now it is acknowledged so the UI can proceed.
func (i *Items) perkIncreaseSlot(session *Session, msg Message) error {
	i.logger.Info(fmt.Sprintf("perk: RequestIncreasePerkSlot body=%x len=%d", msg.Body, len(msg.Body)))
	return session.SendPlain(Message{ID: rmiAnswerIncPerkSlot, Body: msg.Body})
}

func (i *Items) gearEquip(session *Session, msg Message) error {
	return i.attach(session, msg, kindGear, rmiAnswerEquipGearItem, rmiAnswerEquipGearFail)
}

func (i *Items) gearBreak(session *Session, msg Message) error {
	return i.detach(session, msg, kindGear, rmiAnswerBreakGearItem, rmiAnswerBreakGearFail)
}

func (i *Items) equipCustom(session *Session, msg Message) error {
	return i.attach(session, msg, kindCustom, rmiAnswerEquipCustomItem, rmiAnswerEquipCustomFail)
}

func (i *Items) breakCustom(session *Session, msg Message) error {
	return i.detach(session, msg, kindCustom, rmiAnswerBreakCustomItem, rmiAnswerBreakCustomFail)
}

func (i *Items) attach(session *Session, msg Message, kind string, okID, failID uint16) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("%s attach before authentication", kind)
	}
	if len(msg.Body) < 4 {
		i.logger.Info(fmt.Sprintf("%s: attach body too short: %x", kind, msg.Body))
		return session.SendPlain(Message{ID: failID, Body: int32LE(0)})
	}
	parentSerial := binary.LittleEndian.Uint16(msg.Body[:2])
	childSerial := binary.LittleEndian.Uint16(msg.Body[2:4])

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	updated, child, err := i.players.Attach(ctx, player.SteamID, parentSerial, childSerial)
	if err != nil {
		i.logger.Info(fmt.Sprintf("%s: attach %d->%d rejected: %v", kind, childSerial, parentSerial, err))
		return session.SendPlain(Message{ID: failID, Body: int32LE(0)})
	}
	updatePlayer(session, updated)

	if err := i.sendAttachment(session, parentSerial, child, slotActive); err != nil {
		return err
	}
	i.logger.Info(fmt.Sprintf("%s: attached serial=%d item=%d slot=0x%02x to parent=%d",
		kind, childSerial, child.ItemIndex, child.ItemType, parentSerial))
	return session.SendPlain(Message{ID: okID, Body: msg.Body[:4]})
}

func (i *Items) detach(session *Session, msg Message, kind string, okID, failID uint16) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("%s detach before authentication", kind)
	}
	if len(msg.Body) < 4 {
		i.logger.Info(fmt.Sprintf("%s: detach body too short: %x", kind, msg.Body))
		return session.SendPlain(Message{ID: failID, Body: int32LE(0)})
	}
	parentSerial := binary.LittleEndian.Uint16(msg.Body[:2])
	childSerial := binary.LittleEndian.Uint16(msg.Body[2:4])

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	updated, child, err := i.players.Detach(ctx, player.SteamID, parentSerial, childSerial)
	if err != nil {
		i.logger.Info(fmt.Sprintf("%s: detach %d from %d rejected: %v", kind, childSerial, parentSerial, err))
		return session.SendPlain(Message{ID: failID, Body: int32LE(0)})
	}
	updatePlayer(session, updated)

	if err := i.sendAttachment(session, parentSerial, child, slotInactive); err != nil {
		return err
	}
	i.logger.Info(fmt.Sprintf("%s: detached serial=%d slot=0x%02x from parent=%d", kind, childSerial, child.ItemType, parentSerial))
	return session.SendPlain(Message{ID: okID, Body: msg.Body[:4]})
}

func (i *Items) sendAttachment(session *Session, parentSerial uint16, child models.InventoryItem, active byte) error {
	mounted := models.AttachedItem{ParentSerial: parentSerial, ChildSerial: child.Serial, Slot: child.ItemType}
	if models.IsCustomSlot(child.ItemType) {
		return session.SendPlain(Message{ID: rmiSendEquip, Body: customEquipRow(mounted, active)})
	}
	return session.SendPlain(Message{ID: rmiSendSlot, Body: slotRow(mounted, child.ItemIndex, active)})
}

func customEquipRow(mounted models.AttachedItem, active byte) []byte {
	body := appendUint16(nil, mounted.ChildSerial)
	body = appendUint16(body, mounted.ParentSerial)
	return append(body, mounted.Slot, active)
}

func slotRow(mounted models.AttachedItem, childItemIndex uint32, active byte) []byte {
	body := appendUint16(nil, mounted.ParentSerial)
	body = appendUint16(body, mounted.ChildSerial)
	body = appendUint32(body, childItemIndex)
	return append(body, mounted.Slot, active)
}

func (i *Items) buyCustom(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("buy custom before authentication")
	}
	if len(msg.Body) < 6 {
		return session.SendPlain(Message{ID: rmiAnswerBuyCustomItemFail, Body: int32LE(0)})
	}
	weaponSerial := binary.LittleEndian.Uint16(msg.Body[:2])
	itemIndex := binary.LittleEndian.Uint32(msg.Body[2:6])

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := i.players.BuyCustomPart(ctx, player.SteamID, weaponSerial, itemIndex)
	if err != nil {
		i.logger.Info(fmt.Sprintf("custom: buy rejected part=%d: %v", itemIndex, err))
		return session.SendPlain(Message{ID: rmiAnswerBuyCustomItemFail, Body: int32LE(0)})
	}
	updatePlayer(session, result.Player)
	i.logger.Info(fmt.Sprintf("custom: bought part=%d serial=%d mounted on weapon=%d slot=0x%02x point=%d cash=%d",
		itemIndex, result.Item.Serial, weaponSerial, result.Item.ItemType, result.Player.Point, result.Player.Cash))

	// The part is mounted by the purchase itself, so the slot goes out with the
	// currency/item deltas — that is what makes it show up on the weapon.
	if err := i.completeDeltas(session, result); err != nil {
		return err
	}
	if err := i.sendAttachment(session, weaponSerial, result.Item, slotActive); err != nil {
		return err
	}
	ack := appendUint16(nil, result.Item.Serial)
	ack = appendUint32(ack, itemIndex)
	return session.SendPlain(Message{ID: rmiAnswerBuyCustomItem, Body: ack})
}

func (i *Items) completePurchase(session *Session, result services.BuyResult, answerID uint16) error {
	if err := i.completeDeltas(session, result); err != nil {
		return err
	}
	return session.SendPlain(Message{ID: answerID, Body: appendUint16(nil, result.Item.Serial)})
}

func (i *Items) completeDeltas(session *Session, result services.BuyResult) error {
	switch result.Wallet {
	case models.WalletPoint:
		if err := session.SendPlain(Message{ID: rmiSendPoint, Body: uint64LE(result.Player.Point)}); err != nil {
			return err
		}
	case models.WalletCash:
		if err := session.SendPlain(Message{ID: rmiSendCash, Body: uint64LE(result.Player.Cash)}); err != nil {
			return err
		}
	}
	if result.IsNew {
		if err := session.SendPlain(Message{ID: rmiSendItem, Body: appendItem(nil, result.Item, i.players.ItemFunctionType(result.Item.ItemIndex))}); err != nil {
			return err
		}
	} else {
		if err := session.SendPlain(Message{ID: rmiSendItemValue, Body: itemValueRow(result.Item)}); err != nil {
			return err
		}
	}
	return nil
}

// itemValueRow builds SendItemValue (0x76FE): serial, value, state. For a rental
// the value is the seconds left, so extending one refreshes the client's timer.
func itemValueRow(item models.InventoryItem) []byte {
	row := appendUint16(nil, item.Serial)
	row = appendUint32(row, item.WireValue(time.Now().UTC()))
	return append(row, item.State)
}

func (i *Items) buy(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("buy before authentication")
	}
	if len(msg.Body) < 4 {
		return session.SendPlain(Message{ID: rmiAnswerBuyItemFail, Body: int32LE(0)})
	}
	itemIndex := binary.LittleEndian.Uint32(msg.Body[:4])

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := i.players.Buy(ctx, player.SteamID, itemIndex)
	if err != nil {
		i.logger.Info(fmt.Sprintf("store: buy rejected item=%d: %v", itemIndex, err))
		return session.SendPlain(Message{ID: rmiAnswerBuyItemFail, Body: int32LE(0)})
	}
	updatePlayer(session, result.Player)
	i.logger.Info(fmt.Sprintf("store: bought item=%d serial=%d new=%t price=%d point=%d cash=%d", itemIndex, result.Item.Serial, result.IsNew, result.Price, result.Player.Point, result.Player.Cash))
	return i.completePurchase(session, result, rmiAnswerBuyItem)
}

func (i *Items) equip(session *Session, msg Message) error {
	if len(msg.Body) < 2 {
		return fmt.Errorf("equip request too short: %d", len(msg.Body))
	}
	serial := binary.LittleEndian.Uint16(msg.Body[:2])
	// Serial 0 is the store's "equip what I just bought" prompt, not wired yet.
	if serial == 0 {
		return session.SendPlain(Message{ID: rmiAnswerEquipItem, Body: appendUint16(nil, 0)})
	}
	return i.equipSerial(session, serial, rmiAnswerEquipItem)
}

func (i *Items) equipSerial(session *Session, serial, answerID uint16) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("equip before authentication")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	updated, err := i.players.Equip(ctx, player.SteamID, serial)
	if err != nil {
		return fmt.Errorf("equip serial %d: %w", serial, err)
	}
	updatePlayer(session, updated)

	item, _ := updated.Item(serial)
	slot := appendUint16(nil, item.Serial)
	slot = appendUint32(slot, item.ItemIndex)
	slot = append(slot, item.ItemType)
	if err := session.SendPlain(Message{ID: rmiSendEquipSlot, Body: slot}); err != nil {
		return err
	}
	i.logger.Info(fmt.Sprintf("equip: serial=%d item=%d slot=0x%02x", serial, item.ItemIndex, item.ItemType))
	return session.SendPlain(Message{ID: answerID, Body: appendUint16(nil, serial)})
}

func (i *Items) craft(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("craft before authentication")
	}
	if len(msg.Body) < 4 {
		return i.failCraft(session)
	}
	recipeID := binary.LittleEndian.Uint32(msg.Body[:4])

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := i.players.Craft(ctx, player.SteamID, recipeID)
	if err != nil {
		i.logger.Info(fmt.Sprintf("craft: rejected recipe=%d: %v", recipeID, err))
		return i.failCraft(session)
	}
	updatePlayer(session, result.Player)

	for _, serial := range result.Deleted {
		if err := session.SendPlain(Message{ID: rmiSendDeleteItem, Body: appendUint16(nil, serial)}); err != nil {
			return err
		}
	}
	for _, item := range result.Updated {
		if err := session.SendPlain(Message{ID: rmiSendItemValue, Body: itemValueRow(item)}); err != nil {
			return err
		}
	}
	if result.PricePoint > 0 {
		if err := session.SendPlain(Message{ID: rmiSendPoint, Body: uint64LE(result.Player.Point)}); err != nil {
			return err
		}
	}
	if err := session.SendPlain(Message{ID: rmiSendItem, Body: appendItem(nil, result.Output, i.players.ItemFunctionType(result.Output.ItemIndex))}); err != nil {
		return err
	}

	i.logger.Info(fmt.Sprintf("craft: recipe=%d output=%d serial=%d consumed=%d point=%d",
		recipeID, result.Output.ItemIndex, result.Output.Serial, len(result.Deleted)+len(result.Updated), result.Player.Point))

	ack := appendUint32(nil, recipeID)
	ack = appendUint32(ack, result.Output.ItemIndex)
	return session.SendPlain(Message{ID: rmiAnswerRecipeItem, Body: ack})
}

// TODO: the failure code is 0, which the client renders as the generic "An
// unspecified error occurred" (a Loki.strdb message) — it reads like a backend
// fault. The real per-reason codes (insufficient materials / level / funds) are a
// numeric enum in the original GameServer's HANGAR_RECIPE_ITEM handler, still
// unmapped for now
func (i *Items) failCraft(session *Session) error { //TODO error enum
	return session.SendPlain(Message{ID: rmiAnswerRecipeItemFail, Body: int32LE(0)})
}
