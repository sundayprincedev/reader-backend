package storage

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Client struct {
	client   *mongo.Client
	database *mongo.Database
}

func Connect(ctx context.Context, uri, database string) (*Client, error) {
	opts := options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(10 * time.Second).
		SetTimeout(15 * time.Second)

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return &Client{client: client, database: client.Database(database)}, nil
}

func (c *Client) Collection(name string) *mongo.Collection {
	return c.database.Collection(name)
}

func (c *Client) Bucket() *mongo.GridFSBucket {
	return c.database.GridFSBucket()
}

func (c *Client) EnsureIndexes(ctx context.Context) error {
	if _, err := c.Collection("books").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "owner", Value: 1}, {Key: "key", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "owner", Value: 1}, {Key: "updatedAt", Value: -1}},
		},
	}); err != nil {
		return err
	}

	_, err := c.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, nil)
}

func (c *Client) Disconnect(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}
