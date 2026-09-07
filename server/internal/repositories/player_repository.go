package repositories

import (
	"context"
	"errors"
	"time"

	"go-service-template/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ErrAlreadyExists is returned by Create when the SteamID is already taken (a concurrent login won the race).
var ErrAlreadyExists = errors.New("player already exists")

type PlayerRepository interface {
	Find(ctx context.Context, steamID string) (models.Player, bool, error)
	FindMany(ctx context.Context, steamIDs []string) (map[string]models.Player, error)
	Create(ctx context.Context, player models.Player) error
	Touch(ctx context.Context, steamID string, at time.Time) error
	SaveLoadout(ctx context.Context, steamID string, loadout models.Loadout) error
	CommitInventory(ctx context.Context, steamID string, inventory []models.InventoryItem, wallet models.Wallet, price uint64) (bool, error)
	SetName(ctx context.Context, steamID, name string) error
	FindByName(ctx context.Context, name string) (models.Player, bool, error)
	SaveSocial(ctx context.Context, steamID string, friends []models.Friend, requests []models.FriendRequest) error
	SetClanName(ctx context.Context, steamID, clanName string) error
	SaveAttachments(ctx context.Context, steamID string, attachments models.Attachments) error
}

type MongoPlayerRepository struct {
	collection *mongo.Collection
}

func NewMongoPlayerRepository(collection *mongo.Collection) *MongoPlayerRepository {
	return &MongoPlayerRepository{collection: collection}
}

func (r *MongoPlayerRepository) Find(ctx context.Context, steamID string) (models.Player, bool, error) {
	var player models.Player
	err := r.collection.FindOne(ctx, bson.M{"_id": steamID}).Decode(&player)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.Player{}, false, nil
	}
	if err != nil {
		return models.Player{}, false, err
	}
	return player, true, nil
}

func (r *MongoPlayerRepository) FindMany(ctx context.Context, steamIDs []string) (map[string]models.Player, error) {
	found := make(map[string]models.Player, len(steamIDs))
	if len(steamIDs) == 0 {
		return found, nil
	}
	cursor, err := r.collection.Find(ctx, bson.M{"_id": bson.M{"$in": steamIDs}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var players []models.Player
	if err := cursor.All(ctx, &players); err != nil {
		return nil, err
	}
	for _, player := range players {
		found[player.SteamID] = player
	}
	return found, nil
}

func (r *MongoPlayerRepository) Create(ctx context.Context, player models.Player) error {
	if _, err := r.collection.InsertOne(ctx, player); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *MongoPlayerRepository) Touch(ctx context.Context, steamID string, at time.Time) error {
	_, err := r.collection.UpdateByID(ctx, steamID, bson.M{"$set": bson.M{"last_login": at}})
	return err
}

func (r *MongoPlayerRepository) SaveLoadout(ctx context.Context, steamID string, loadout models.Loadout) error {
	_, err := r.collection.UpdateByID(ctx, steamID, bson.M{"$set": bson.M{"loadout": loadout}})
	return err
}

func (r *MongoPlayerRepository) CommitInventory(ctx context.Context, steamID string, inventory []models.InventoryItem, wallet models.Wallet, price uint64) (bool, error) {
	filter := bson.M{"_id": steamID}
	update := bson.M{"$set": bson.M{"inventory": inventory}}
	if price > 0 {
		field := walletField(wallet)
		filter[field] = bson.M{"$gte": price}
		update["$inc"] = bson.M{field: -int64(price)}
	}
	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}

func (r *MongoPlayerRepository) SaveAttachments(ctx context.Context, steamID string, attachments models.Attachments) error {
	_, err := r.collection.UpdateByID(ctx, steamID, bson.M{"$set": bson.M{"attachments": attachments}})
	return err
}

func (r *MongoPlayerRepository) FindByName(ctx context.Context, name string) (models.Player, bool, error) {
	var player models.Player
	err := r.collection.FindOne(ctx, bson.M{"name": name}).Decode(&player)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.Player{}, false, nil
	}
	if err != nil {
		return models.Player{}, false, err
	}
	return player, true, nil
}

func (r *MongoPlayerRepository) SaveSocial(ctx context.Context, steamID string, friends []models.Friend, requests []models.FriendRequest) error {
	_, err := r.collection.UpdateByID(ctx, steamID, bson.M{"$set": bson.M{
		"friends":         friends,
		"friend_requests": requests,
	}})
	return err
}

func (r *MongoPlayerRepository) SetName(ctx context.Context, steamID, name string) error {
	_, err := r.collection.UpdateByID(ctx, steamID, bson.M{"$set": bson.M{"name": name}})
	return err
}

func (r *MongoPlayerRepository) SetClanName(ctx context.Context, steamID, clanName string) error {
	update := bson.M{"$set": bson.M{"clan_name": clanName}}
	if clanName == "" {
		update = bson.M{"$unset": bson.M{"clan_name": ""}}
	}
	_, err := r.collection.UpdateByID(ctx, steamID, update)
	return err
}

func walletField(w models.Wallet) string {
	if w == models.WalletCash {
		return "cash"
	}
	return "point"
}
