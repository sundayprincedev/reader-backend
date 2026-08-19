package storage

import (
	"context"
	"errors"
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
	books := c.Collection("books")

	for _, stale := range []string{"owner_1_key_1", "owner_1_updatedAt_-1"} {
		if err := books.Indexes().DropOne(ctx, stale); err != nil && !isMissingIndex(err) {
			return err
		}
	}

	_, err := books.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "key", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "updatedAt", Value: -1}},
		},
	})
	return err
}

func isMissingIndex(err error) bool {
	var serverErr mongo.ServerError
	return errors.As(err, &serverErr) && serverErr.HasErrorCode(27)
}

func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, nil)
}

func (c *Client) Disconnect(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}
