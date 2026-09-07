package catalog

import (
	"bytes"
	"embed"
	"encoding/csv"
	"sort"
	"strconv"
	"strings"
)

//go:embed data
var files embed.FS

// starterItems is the kit granted to a brand-new account: one character per
// faction (required, or the empty faction shows "invalid data") plus a basic
// primary weapon. Each item's slot is derived from its catalog groups.
var StarterItems = []uint32{
	11100101, // Benedict — A.T.C. character
	12100101, // Jack Conrad — T.F. character
	21102201, // M16 — primary weapon
}

// Catalog is the static reference data extracted from the game's .Arch01 files
// (see scripts/extract_catalog.py). Only gameplay-relevant columns are modelled;
// the raw CSVs carry more (marketing timers, descriptions).
type Catalog struct {
	Items     map[uint32]GameItem
	Maps      map[uint32]MapInfo
	Rewards   map[uint32][]RewardItem
	Classes   []ClassInfo
	Recipes   map[uint32]Recipe
	Functions map[uint32]FunctionItem
	PartsFor  map[uint32]map[uint32]bool
	Parts     map[uint32]CustomPart
}

// CustomPart is a row of CustomPartItem.csv: the stat deltas a part applies and
// the modes it is allowed in. AbleToMode is a per-mode flag string ("111111"),
// kept as read since the mode order it indexes is not established.
type CustomPart struct {
	PartIndex  uint32
	Attack     int
	Accuracy   int
	Stability  int
	ShotSpeed  int
	AbleToMode string
}

// Accepts reports whether a weapon takes a given part. A weapon absent from
// PartLinkTable.csv accepts nothing: the table is the whitelist, so an unknown
// weapon is an unanswered question, not permission.
func (c *Catalog) Accepts(weaponIndex, partIndex uint32) bool {
	return c.PartsFor[weaponIndex][partIndex]
}

// FunctionItem is a row of FunctionItem.csv (r3.Arch01), keyed by FunctionIndex.
// A GameItem points at one through its FunctionIndex, and the FunctionType is
// what travels in the inventory wire row.
type FunctionItem struct {
	FunctionIndex uint32
	FunctionType  int
	ItemDesc      string
	Value         int
}

func (c *Catalog) FunctionType(itemIndex uint32) byte {
	item, ok := c.Items[itemIndex]
	if !ok || item.FunctionIndex == 0 {
		return 0
	}
	function, ok := c.Functions[uint32(item.FunctionIndex)]
	if !ok || function.FunctionType < 0 || function.FunctionType > 0xFF {
		return 0
	}
	return byte(function.FunctionType)
}

// GameItem is a row of GameItem.csv (d3.Arch01), keyed by ItemIndex. It is the
// static definition an inventory item's ItemIndex points at.
type GameItem struct {
	ItemIndex     uint32
	RecordName    string
	EngName       string
	HighGroup     int
	MiddleGroup   int
	FunctionIndex int

	PointPrice  uint64
	CashPrice   uint64
	BonusPoint  uint64
	CashType    int
	IsResell    bool
	ResellPoint uint64
	IsSell      bool
	IsShopShow  bool
	IsSale      bool
	SalePrice   uint64
	RepairPoint uint64
	CanGift     bool

	UseType    int
	UseTime    int
	UseCount   int
	Durability int
	IsMerge    bool
	OnlyPvE    bool

	BuyLevelLimit   int
	EquipLevelLimit int
	WeaponType      int
	SlotCount       int
	CapsuleGrade    int

	Attack    int
	Accuracy  int
	Stability int
	ShotSpeed int
	Carry     int
	Magazine  int
	Supply    int
}

// MapInfo is a row of MapInfo.csv (e3.Arch01), keyed by MapIndex.
type MapInfo struct {
	MapIndex        uint32
	MapMode         int
	PlayNormal      bool
	PlayPvE         bool
	User            []int // room-size options (pipe-separated in the CSV)
	UserMin         int
	UserDefault     int
	MapName         string
	ModeName        string
	Difficulty      int
	WinRewardItem   uint32
	LoseRewardIndex uint32
	DrawRewardIndex uint32
}

var installedOpenMaps = map[string]bool{
	"TAM_Rooftop": true, "TAM_Rooftop_Light": true, "TAM_Killhouse": true,
	"TAM_Mansion": true, "TAM_Arsenal": true, "TAM_RuinCity": true,
	"TAM_Gore_Mansion": true, "SSM_Downfall": true, "TDM_PentHouse": true,
	"TDM__Laboratory": true, "TDM_Airport": true, "TCP_EPA_TH_01": true,
	"PVE_Act01Ms02_E": true, "PVE_Act01Ms02_N": true, "PVE_Act01Ms02_H": true,
	"PVE_Runaway_E": true, "PVE_Runaway_N": true, "PVE_Runaway_H": true,
	"PVE_Runaway02_E": true, "PVE_Runaway02_N": true, "PVE_Runaway02_H": true,
	"Rag_Town": true, "Rag_Retake": true,
}

// TODO static value
// OpenMaps returns the installed, advertisable maps sorted by MapIndex.
func (c *Catalog) OpenMaps() []MapInfo {
	maps := make([]MapInfo, 0, len(installedOpenMaps))
	for _, info := range c.Maps {
		if installedOpenMaps[info.MapName] {
			maps = append(maps, info)
		}
	}
	sort.Slice(maps, func(i, j int) bool { return maps[i].MapIndex < maps[j].MapIndex })
	return maps
}

// RewardItem is a row of RewardItem.csv (i3.Arch01). RewardIndex is not unique:
// a match result selects every row of the chosen index.
type RewardItem struct {
	RewardIndex uint32
	Min         int
	Max         int
	RandomMax   int
	RewardItem  uint32
	RewardExp   uint64
	RewardPoint uint64
	RewardType  int
}

// ClassInfo is a row of ClassInfo.csv (a3.Arch01). Progression resolves the row
// whose [ExpMin, ExpMax] contains the player's exp.
type ClassInfo struct {
	ClassIndex      uint32
	ClassLevel      int
	Type            int
	Career          int
	ClassName       string
	ExpMin          uint64
	ExpMax          uint64
	PointValue      uint64
	RewardItemIndex [3]uint32
}

// Recipe is a row of Recipe.csv (j3.Arch01), keyed by id.
type Recipe struct {
	ID             uint32
	Exist          bool
	RepresentIndex uint32
	RequireLevel   int
	PriceType      int
	PricePoint     uint64
	PriceCash      uint64
	Materials      []RecipeMaterial
	ItemOutput     uint32
	CountOutput    int
}

type RecipeMaterial struct {
	ItemIndex uint32
	Count     int
}

// ItemType derives the inventory category / equip slot from the item's groups:
// 0x0b + (HighGroup-1)*0x0a + (MiddleGroup-1). Verified for characters
// (0x0b A.T.C. / 0x0c T.F.) and weapons (0x15 = primary), where itemType == slot.
func (g GameItem) ItemType() uint8 {
	return uint8(0x0B + (g.HighGroup-1)*0x0A + (g.MiddleGroup - 1))
}

func Load() (*Catalog, error) {
	catalog := &Catalog{
		Items:     make(map[uint32]GameItem),
		Maps:      make(map[uint32]MapInfo),
		Rewards:   make(map[uint32][]RewardItem),
		Recipes:   make(map[uint32]Recipe),
		Functions: make(map[uint32]FunctionItem),
		PartsFor:  make(map[uint32]map[uint32]bool),
		Parts:     make(map[uint32]CustomPart),
	}

	if err := each("data/GameItem.csv", func(r record) {
		item := gameItemFromRow(r)
		catalog.Items[item.ItemIndex] = item
	}); err != nil {
		return nil, err
	}
	if err := each("data/MapInfo.csv", func(r record) {
		mapInfo := mapInfoFromRow(r)
		catalog.Maps[mapInfo.MapIndex] = mapInfo
	}); err != nil {
		return nil, err
	}
	if err := each("data/RewardItem.csv", func(r record) {
		reward := rewardItemFromRow(r)
		catalog.Rewards[reward.RewardIndex] = append(catalog.Rewards[reward.RewardIndex], reward)
	}); err != nil {
		return nil, err
	}
	if err := each("data/ClassInfo.csv", func(r record) {
		catalog.Classes = append(catalog.Classes, classInfoFromRow(r))
	}); err != nil {
		return nil, err
	}
	if err := each("data/Recipe.csv", func(r record) {
		recipe := recipeFromRow(r)
		catalog.Recipes[recipe.ID] = recipe
	}); err != nil {
		return nil, err
	}
	if err := each("data/PartLinkTable.csv", func(r record) {
		weapon, part := r.u32("WeaponIndex"), r.u32("PartIndex")
		if catalog.PartsFor[weapon] == nil {
			catalog.PartsFor[weapon] = make(map[uint32]bool)
		}
		catalog.PartsFor[weapon][part] = true
	}); err != nil {
		return nil, err
	}
	if err := each("data/CustomPartItem.csv", func(r record) {
		part := CustomPart{
			PartIndex:  r.u32("PartIndex"),
			Attack:     r.int("Attack"),
			Accuracy:   r.int("Accuracy"),
			Stability:  r.int("Stability"),
			ShotSpeed:  r.int("ShotSpeed"),
			AbleToMode: r.str("AbleToMode"),
		}
		catalog.Parts[part.PartIndex] = part
	}); err != nil {
		return nil, err
	}
	if err := each("data/FunctionItem.csv", func(r record) {
		function := FunctionItem{
			FunctionIndex: uint32(r.int("FunctionIndex")),
			FunctionType:  r.int("FunctionType"),
			ItemDesc:      r.str("ItemDesc"),
			Value:         r.int("Value"),
		}
		catalog.Functions[function.FunctionIndex] = function
	}); err != nil {
		return nil, err
	}
	return catalog, nil
}

func gameItemFromRow(r record) GameItem {
	return GameItem{
		ItemIndex:       r.u32("ItemIndex"),
		RecordName:      r.str("RecordName"),
		EngName:         r.str("EngName"),
		HighGroup:       r.int("HighGroup"),
		MiddleGroup:     r.int("MiddleGroup"),
		FunctionIndex:   r.int("FunctionIndex"),
		PointPrice:      r.u64("PointPrice"),
		CashPrice:       r.u64("CashPrice"),
		BonusPoint:      r.u64("BonusPoint"),
		CashType:        r.int("CashType"),
		IsResell:        r.bool("IsResell"),
		ResellPoint:     r.u64("ResellPoint"),
		IsSell:          r.bool("IsSell"),
		IsShopShow:      r.bool("IsShopShow"),
		IsSale:          r.bool("IsSale"),
		SalePrice:       r.u64("SalePrice"),
		RepairPoint:     r.u64("RepairPoint"),
		CanGift:         r.bool("Cangift"),
		UseType:         r.int("UseType"),
		UseTime:         r.int("UseTime"),
		UseCount:        r.int("UseCount"),
		Durability:      r.int("Durability"),
		IsMerge:         r.bool("IsMerge"),
		OnlyPvE:         r.bool("OnlyPvE"),
		BuyLevelLimit:   r.int("BuyLevelLimit"),
		EquipLevelLimit: r.int("EquipLevelLimit"),
		WeaponType:      r.int("WeaponType"),
		SlotCount:       r.int("SlotCount"),
		CapsuleGrade:    r.int("CapsuleGrade"),
		Attack:          r.int("Attack"),
		Accuracy:        r.int("Accuracy"),
		Stability:       r.int("Stability"),
		ShotSpeed:       r.int("ShotSpeed"),
		Carry:           r.int("Carry"),
		Magazine:        r.int("Magazine"),
		Supply:          r.int("Supply"),
	}
}

func mapInfoFromRow(r record) MapInfo {
	return MapInfo{
		MapIndex:        r.u32("MapIndex"),
		MapMode:         r.int("MapMode"),
		PlayNormal:      r.bool("PlayNormal"),
		PlayPvE:         r.bool("PlayPve"),
		User:            r.pipeInts("User"),
		UserMin:         r.int("UserMin"),
		UserDefault:     r.int("UserDefault"),
		MapName:         r.str("MapName"),
		ModeName:        r.str("ModeName"),
		Difficulty:      r.int("Difficulty"),
		WinRewardItem:   r.u32("WinRewardItem"),
		LoseRewardIndex: r.u32("LoseRewardIndex"),
		DrawRewardIndex: r.u32("DrawRewardIndex"),
	}
}

func rewardItemFromRow(r record) RewardItem {
	return RewardItem{
		RewardIndex: r.u32("RewardIndex"),
		Min:         r.int("Min"),
		Max:         r.int("Max"),
		RandomMax:   r.int("RandomMax"),
		RewardItem:  r.u32("RewardItem"),
		RewardExp:   r.u64("RewardExp"),
		RewardPoint: r.u64("RewardPoint"),
		RewardType:  r.int("RewardType"),
	}
}

func classInfoFromRow(r record) ClassInfo {
	return ClassInfo{
		ClassIndex: r.u32("ClassIndex"),
		ClassLevel: r.int("ClassLevel"),
		Type:       r.int("Type"),
		Career:     r.int("Career"),
		ClassName:  r.str("ClassName"),
		ExpMin:     r.u64("ExpMin"),
		ExpMax:     r.u64("ExpMax"),
		PointValue: r.u64("PointValue"),
		RewardItemIndex: [3]uint32{
			r.u32("RewardItemIndex1"),
			r.u32("RewardItemIndex2"),
			r.u32("RewardItemIndex3"),
		},
	}
}

func recipeFromRow(r record) Recipe {
	recipe := Recipe{
		ID:             r.u32("id"),
		Exist:          r.bool("exist"),
		RepresentIndex: r.u32("RepresentIndex"),
		RequireLevel:   r.int("requireLv"),
		PriceType:      r.int("priceType"),
		PricePoint:     r.u64("priceP"),
		PriceCash:      r.u64("priceC"),
		ItemOutput:     r.u32("itemOutput"),
		CountOutput:    r.int("countOutput"),
	}
	for i := 1; i <= 6; i++ {
		suffix := strconv.Itoa(i)
		itemIndex := r.u32("Material" + suffix)
		if itemIndex == 0 {
			continue
		}
		recipe.Materials = append(recipe.Materials, RecipeMaterial{
			ItemIndex: itemIndex,
			Count:     r.int("mCount" + suffix),
		})
	}
	return recipe
}

// record is a CSV row addressable by column name.
type record struct {
	index  map[string]int
	fields []string
}

func (r record) str(column string) string {
	if i, ok := r.index[column]; ok && i < len(r.fields) {
		return strings.TrimSpace(r.fields[i])
	}
	return ""
}

func (r record) int(column string) int {
	value, _ := strconv.Atoi(r.str(column))
	return value
}

func (r record) u32(column string) uint32 {
	value, _ := strconv.ParseUint(r.str(column), 10, 32)
	return uint32(value)
}

func (r record) u64(column string) uint64 {
	value, _ := strconv.ParseUint(r.str(column), 10, 64)
	return value
}

func (r record) bool(column string) bool {
	return r.str(column) == "1"
}

func (r record) pipeInts(column string) []int {
	raw := r.str(column)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "|")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			values = append(values, n)
		}
	}
	return values
}

func each(name string, fn func(record)) error {
	data, err := files.ReadFile(name)
	if err != nil {
		return err
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	index := make(map[string]int, len(rows[0]))
	for i, header := range rows[0] {
		index[strings.TrimSpace(header)] = i
	}
	for _, fields := range rows[1:] {
		fn(record{index: index, fields: fields})
	}
	return nil
}
