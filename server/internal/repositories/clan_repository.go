package repositories

import (
	"context"
	"errors"
	"regexp"

	"go-service-template/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrClanExists = errors.New("clan already exists")

type ClanRepository interface {
	Find(ctx context.Context, name string) (models.Clan, bool, error)
	Create(ctx context.Context, clan models.Clan) error
	SearchByName(ctx context.Context, name string, limit int64) ([]models.Clan, error)
	SaveApplicants(ctx context.Context, name string, applicants []models.ClanApplicant) error
	SaveMembers(ctx context.Context, name string, members []models.ClanMember) error
	SaveRoster(ctx context.Context, name string, members []models.ClanMember, applicants []models.ClanApplicant) error
	SaveLeadership(ctx context.Context, name, master string, members []models.ClanMember) error
	FindByApplicant(ctx context.Context, steamID string) (models.Clan, bool, error)
	FindByMark(ctx context.Context, mark [3]int32, excludeName string) (models.Clan, bool, error)
	SaveMark(ctx context.Context, name string, mark [3]int32, markExtra uint16) error
	Delete(ctx context.Context, name string) error
}

type MongoClanRepository struct {
	collection *mongo.Collection
}

func NewMongoClanRepository(collection *mongo.Collection) *MongoClanRepository {
	return &MongoClanRepository{collection: collection}
}

func (r *MongoClanRepository) Find(ctx context.Context, name string) (models.Clan, bool, error) {
	var clan models.Clan
	err := r.collection.FindOne(ctx, bson.M{"_id": name}).Decode(&clan)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.Clan{}, false, nil
	}
	if err != nil {
		return models.Clan{}, false, err
	}
	return clan, true, nil
}

func (r *MongoClanRepository) Create(ctx context.Context, clan models.Clan) error {
	if _, err := r.collection.InsertOne(ctx, clan); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrClanExists
		}
		return err
	}
	return nil
}

func (r *MongoClanRepository) FindByMark(ctx context.Context, mark [3]int32, excludeName string) (models.Clan, bool, error) {
	filter := bson.M{"mark": mark}
	if excludeName != "" {
		filter["_id"] = bson.M{"$ne": excludeName}
	}

	var clan models.Clan
	err := r.collection.FindOne(ctx, filter).Decode(&clan)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.Clan{}, false, nil
	}
	if err != nil {
		return models.Clan{}, false, err
	}
	return clan, true, nil
}

func (r *MongoClanRepository) SaveMark(ctx context.Context, name string, mark [3]int32, markExtra uint16) error {
	_, err := r.collection.UpdateByID(ctx, name, bson.M{"$set": bson.M{
		"mark":       mark,
		"mark_extra": markExtra,
	}})
	return err
}

func (r *MongoClanRepository) Delete(ctx context.Context, name string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": name})
	return err
}

func (r *MongoClanRepository) FindByApplicant(ctx context.Context, steamID string) (models.Clan, bool, error) {
	var clan models.Clan
	err := r.collection.FindOne(ctx, bson.M{"applicants.steam_id": steamID}).Decode(&clan)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.Clan{}, false, nil
	}
	if err != nil {
		return models.Clan{}, false, err
	}
	return clan, true, nil
}

func (r *MongoClanRepository) SaveApplicants(ctx context.Context, name string, applicants []models.ClanApplicant) error {
	_, err := r.collection.UpdateByID(ctx, name, bson.M{"$set": bson.M{"applicants": applicants}})
	return err
}

func (r *MongoClanRepository) SaveMembers(ctx context.Context, name string, members []models.ClanMember) error {
	_, err := r.collection.UpdateByID(ctx, name, bson.M{"$set": bson.M{"members": members}})
	return err
}

func (r *MongoClanRepository) SaveRoster(ctx context.Context, name string, members []models.ClanMember, applicants []models.ClanApplicant) error {
	_, err := r.collection.UpdateByID(ctx, name, bson.M{"$set": bson.M{
		"members":    members,
		"applicants": applicants,
	}})
	return err
}

func (r *MongoClanRepository) SaveLeadership(ctx context.Context, name, master string, members []models.ClanMember) error {
	_, err := r.collection.UpdateByID(ctx, name, bson.M{"$set": bson.M{
		"master":  master,
		"members": members,
	}})
	return err
}

func (r *MongoClanRepository) SearchByName(ctx context.Context, name string, limit int64) ([]models.Clan, error) {
	filter := bson.M{"_id": bson.M{"$regex": "^" + regexp.QuoteMeta(name), "$options": "i"}}
	cursor, err := r.collection.Find(ctx, filter, options.Find().SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var clans []models.Clan
	if err := cursor.All(ctx, &clans); err != nil {
		return nil, err
	}
	return clans, nil
}
