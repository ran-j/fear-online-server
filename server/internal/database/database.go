package database

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Database struct {
	client   *mongo.Client
	database *mongo.Database
}

func Connect(uri, name string) (*Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	return &Database{client: client, database: client.Database(name)}, nil
}

func (d *Database) Collection(name string) *mongo.Collection {
	return d.database.Collection(name)
}

func (d *Database) Disconnect() error {
	return d.client.Disconnect(context.Background())
}
