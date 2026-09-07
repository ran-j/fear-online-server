package proudnet

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	"go-service-template/internal/models"
)

const (
	rmiRequestLogin    uint16 = 0x7595
	rmiAnswerLogin     uint16 = 0x75C7
	rmiNotifyCharacter uint16 = 0x75CD
	rmiNotifyRecord    uint16 = 0x75CE

	rmiItemRequestLogin  uint16 = 0x76C1
	rmiItemAnswerLogin   uint16 = 0x76F3
	rmiSendPoint         uint16 = 0x762F
	rmiSendCash          uint16 = 0x7693
	rmiSendItemList      uint16 = 0x76F7
	rmiSendEquipSlotList uint16 = 0x76FF

	rmiMapQuestRequestLogin uint16 = 0x78B5
	rmiMapQuestAnswerLogin  uint16 = 0x78E7
	rmiSendMapQuestList     uint16 = 0x78E9

	mapQuestPvPMap uint16 = 204 // TDM_PentHouse
	mapQuestPvEMap uint16 = 704 // PVE_Act01Ms02_E

	steamAccountBase uint64 = 76561197960265728
)

// TODO remove this later and make something more robust for characterID
func characterID(steamID string) uint64 {
	if id, err := strconv.ParseUint(steamID, 10, 64); err == nil && id > steamAccountBase {
		return id - steamAccountBase
	}
	sum := fnv.New64a()
	sum.Write([]byte(steamID))
	return sum.Sum64()
}

func (h *Handlers) login(session *Session, _ Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("login without an authenticated account")
	}
	pushes := []followUp{
		{rmiNotifyCharacter, notifyCharacter(player)},
		{rmiNotifyRecord, notifyRecord(player)},
	}
	pushes = append(pushes, h.clanPushes(player)...)
	return h.sendResponse(session, response{answer: rmiAnswerLogin, followUps: pushes})
}

func (h *Handlers) clanPushes(player models.Player) []followUp {
	clan, ok := h.playerClan(player)
	if !ok {
		return h.pendingClanPushes(player)
	}
	pushes := []followUp{
		{rmiSendClanInfo, appendClanInfo(nil, clan)},
		{rmiSendMemberList, clanMemberList(h.clanRoster(clan))},
		{rmiNotifyMemberCount, makeClanMemberCounts(clan).row()},
		{rmiNotifyUserClanInfo, notifyUserClanInfoRow(uint16(characterID(player.SteamID)), clan)},
	}
	if requests, ok := h.clanRequests(clan, player.SteamID); ok {
		pushes = append(pushes, followUp{rmiSendRequestList, requests})
	}
	return pushes
}

func (h *Handlers) pendingClanPushes(player models.Player) []followUp {
	if h.clans == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clan, found, err := h.clans.PendingApplication(ctx, player.SteamID)
	if err != nil {
		h.logger.Error(fmt.Sprintf("clan: load pending application of %s: %v", player.Name, err))
		return nil
	}
	if !found {
		return nil
	}
	return []followUp{{rmiSendRequestClanInfo, appendRequestClanInfo(nil, clan)}}
}

func (h *Handlers) clanRequests(clan models.Clan, steamID string) ([]byte, bool) {
	reviewer := false
	for _, member := range clan.Members {
		if member.SteamID == steamID && models.CanReviewClanApplications(member.Rank) {
			reviewer = true
			break
		}
	}
	if !reviewer || len(clan.Applicants) == 0 {
		return nil, false
	}

	steamIDs := make([]string, 0, len(clan.Applicants))
	for _, applicant := range clan.Applicants {
		steamIDs = append(steamIDs, applicant.SteamID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	accounts, err := h.players.FindMany(ctx, steamIDs)
	if err != nil {
		h.logger.Error(fmt.Sprintf("clan: load applicants of %q: %v", clan.Name, err))
		accounts = nil
	}
	return clanRequestList(clan.Applicants, accounts), true
}

// TODO: channel, lobby and room are left at zero until the presence events
// (0xA646-0xA64C) are wired.
func (h *Handlers) clanRoster(clan models.Clan) []clanMemberWire {
	steamIDs := make([]string, 0, len(clan.Members))
	for _, member := range clan.Members {
		steamIDs = append(steamIDs, member.SteamID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	accounts, err := h.players.FindMany(ctx, steamIDs)
	if err != nil {
		h.logger.Error(fmt.Sprintf("clan: load roster of %q: %v", clan.Name, err))
		accounts = nil
	}

	rows := make([]clanMemberWire, 0, len(clan.Members))
	for _, member := range clan.Members {
		presence := clanMemberPresence{Online: h.sessions.IsOnline(member.SteamID)}
		rows = append(rows, makeClanMemberWire(member, accounts[member.SteamID], presence))
	}
	return rows
}

func (h *Handlers) playerClan(player models.Player) (models.Clan, bool) {
	if player.ClanName == "" || h.clans == nil {
		return models.Clan{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clan, found, err := h.clans.Find(ctx, player.ClanName)
	if err != nil {
		h.logger.Error(fmt.Sprintf("clan: load %q during login: %v", player.ClanName, err))
		return models.Clan{}, false
	}
	if !found {
		h.logger.Info(fmt.Sprintf("clan: player %s references missing clan %q", player.Name, player.ClanName))
		return models.Clan{}, false
	}
	return clan, true
}

func (h *Handlers) itemLogin(session *Session, _ Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("item login without an authenticated account")
	}
	err := h.sendResponse(session, response{
		answer: rmiItemAnswerLogin,
		followUps: []followUp{
			{rmiSendPoint, uint64LE(player.Point)},
			{rmiSendCash, uint64LE(player.Cash)},
			{0x76F5, empty}, // SendItemInfoList
			{0x76F6, empty}, // SendDefaultItemList
			{rmiSendItemList, itemList(player.Inventory, h.players.ItemFunctionType)},
			{0x76F9, empty}, // SendDeleteList
			{rmiSendSlotList, slotList(player)},
			{0x76FD, empty}, // SendItemValueList
			{rmiSendEquipSlotList, equipSlotList(player)},
			{rmiSendEquipList, customEquipList(player)},
		},
	})
	if err != nil {
		return err
	}
	h.logger.Info(fmt.Sprintf("login: sent hangar point=%d cash=%d items=%d", player.Point, player.Cash, len(player.Inventory)))
	return nil
}

func (h *Handlers) mapQuestLogin(session *Session, _ Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("map quest login without an authenticated account")
	}
	return h.sendResponse(session, response{
		answer:    rmiMapQuestAnswerLogin,
		followUps: []followUp{{rmiSendMapQuestList, mapQuestList(player)}},
	})
}

func mapQuestList(p models.Player) []byte {
	pvp, pve := p.Records.PvP, p.Records.PvE
	body := EncodeVarInt(2)
	// PvP row: outcome0/1/2 are win/draw/lose, and the window derives games from
	// their sum.
	body = appendMapQuestRow(body, mapQuestPvPMap, mapQuestRow{
		Outcome0: pvp.Win,
		Outcome1: pvp.Draw,
		Outcome2: pvp.Lose,
		Kill:     pvp.Kill,
		Headshot: pvp.Headshot,
		Death:    pvp.Death,
	})
	// PvE row: outcome0 is completed runs, outcome1 the remainder of games played,
	// and kill is the NPC count.
	incomplete := uint32(0)
	if pve.Games > pve.Complete {
		incomplete = pve.Games - pve.Complete
	}
	return appendMapQuestRow(body, mapQuestPvEMap, mapQuestRow{
		Outcome0: pve.Complete,
		Outcome1: incomplete,
		Kill:     pve.NpcKill,
		Death:    pve.Death,
	})
}

type mapQuestRow struct {
	Outcome0 uint32
	Outcome1 uint32
	Outcome2 uint32
	Kill     uint32
	Headshot uint32
	Death    uint32
}

func appendMapQuestRow(dst []byte, mapIndex uint16, row mapQuestRow) []byte {
	dst = appendUint16(dst, mapIndex)
	for _, value := range []uint32{
		row.Outcome0, row.Outcome1, row.Outcome2,
		row.Kill, row.Headshot, 0, row.Death,
		0, 0, 0, 0,
	} {
		dst = appendUint32(dst, value)
	}
	return dst
}

// notifyCharacter builds NotifyCharacter (0x75CD): identity, progression and the aggregate PvP/PvE/Hit records.
func notifyCharacter(p models.Player) []byte {
	body := WriteProudString(nil, p.Name)
	body = appendUint64(body, characterID(p.SteamID))
	body = append(body, levelByte(p))
	body = appendUint32(body, uint32(p.Level))
	body = appendUint32(body, uint32(p.Exp))
	body = appendPvP(body, p.Records.PvP)
	body = appendPvE(body, p.Records.PvE)
	body = appendHit(body, p.Records.Hit)
	return appendUint32(body, 0) // record tail
}

// notifyRecord builds NotifyRecord (0x75CE): the aggregates the BASIC tab reads.
func notifyRecord(p models.Player) []byte {
	body := appendUint64(nil, characterID(p.SteamID))
	body = append(body, levelByte(p))
	body = appendPvP(body, p.Records.PvP)
	body = appendPvE(body, p.Records.PvE)
	return appendUint32(body, 0) // record tail
}

func appendItem(dst []byte, item models.InventoryItem, functionCode byte) []byte {
	dst = appendUint16(dst, item.Serial)
	dst = appendUint32(dst, item.ItemIndex)
	dst = appendUint32(dst, item.WireValue(time.Now().UTC()))
	return append(dst, item.State, item.InstanceAux, item.ItemType, functionCode)
}

// itemList builds SendItemList (0x76F7): every owned item instance.
func itemList(items []models.InventoryItem, functionOf func(uint32) byte) []byte {
	body := EncodeVarInt(len(items))
	for _, item := range items {
		body = appendItem(body, item, functionOf(item.ItemIndex))
	}
	return body
}

func slotList(p models.Player) []byte {
	indexBySerial := make(map[uint16]uint32, len(p.Inventory))
	for _, item := range p.Inventory {
		indexBySerial[item.Serial] = item.ItemIndex
	}
	mounts := make([]models.AttachedItem, 0, len(p.Attachments))
	for _, mounted := range p.Attachments {
		if !models.IsCustomSlot(mounted.Slot) {
			mounts = append(mounts, mounted)
		}
	}
	body := EncodeVarInt(len(mounts))
	for _, mounted := range mounts {
		body = append(body, slotRow(mounted, indexBySerial[mounted.ChildSerial], slotActive)...)
	}
	return body
}

func customEquipList(p models.Player) []byte {
	mounts := make([]models.AttachedItem, 0, len(p.Attachments))
	for _, mounted := range p.Attachments {
		if models.IsCustomSlot(mounted.Slot) {
			mounts = append(mounts, mounted)
		}
	}
	body := EncodeVarInt(len(mounts))
	for _, mounted := range mounts {
		body = append(body, customEquipRow(mounted, slotActive)...)
	}
	return body
}

func equipSlotList(p models.Player) []byte {
	indexBySerial := make(map[uint16]uint32, len(p.Inventory))
	for _, item := range p.Inventory {
		indexBySerial[item.Serial] = item.ItemIndex
	}
	body := EncodeVarInt(len(p.Loadout))
	for _, equipped := range p.Loadout {
		body = appendUint16(body, equipped.Serial)
		body = appendUint32(body, indexBySerial[equipped.Serial])
		body = append(body, equipped.Slot)
	}
	return body
}

func appendPvP(dst []byte, r models.PvPRecord) []byte {
	games := r.Games
	if games == 0 && r.Win+r.Lose > 0 {
		games = r.Win + r.Lose
	}
	for _, value := range []uint32{games, r.Win, r.Lose, r.Kill, r.Death, r.Headshot} {
		dst = appendUint32(dst, value)
	}
	return dst
}

func appendPvE(dst []byte, r models.PvERecord) []byte {
	for _, value := range []uint32{r.Complete, r.Games, r.NpcKill, r.Death} {
		dst = appendUint32(dst, value)
	}
	return dst
}

func appendHit(dst []byte, r models.HitRecord) []byte {
	for _, value := range []uint32{r.Head, r.Arms, r.Body, r.Legs, r.Miss} {
		dst = appendUint32(dst, value)
	}
	return dst
}

func levelByte(p models.Player) byte {
	if p.Level == 0 {
		return 1
	}
	if p.Level > 0xFF {
		return 0xFF
	}
	return byte(p.Level)
}
