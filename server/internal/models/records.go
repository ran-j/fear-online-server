package models

type Records struct {
	PvP PvPRecord `bson:"pvp" json:"pvp"`
	PvE PvERecord `bson:"pve" json:"pve"`
	Hit HitRecord `bson:"hit" json:"hit"`
}

type PvPRecord struct {
	Games    uint32 `bson:"games" json:"games"`
	Win      uint32 `bson:"win" json:"win"`
	Lose     uint32 `bson:"lose" json:"lose"`
	Draw     uint32 `bson:"draw,omitempty" json:"draw,omitempty"`
	Kill     uint32 `bson:"kill" json:"kill"`
	Death    uint32 `bson:"death" json:"death"`
	Headshot uint32 `bson:"headshot" json:"headshot"`
}

type PvERecord struct {
	Complete uint32 `bson:"complete" json:"complete"`
	Games    uint32 `bson:"games" json:"games"`
	NpcKill  uint32 `bson:"npc_kill" json:"npcKill"`
	Death    uint32 `bson:"death" json:"death"`
}

type HitRecord struct {
	Head uint32 `bson:"head" json:"head"`
	Arms uint32 `bson:"arms" json:"arms"`
	Body uint32 `bson:"body" json:"body"`
	Legs uint32 `bson:"legs" json:"legs"`
	Miss uint32 `bson:"miss" json:"miss"`
}
