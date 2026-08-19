package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/sundayprincedev/mereader/internal/models"
	"github.com/sundayprincedev/mereader/internal/storage"
)

var ErrNotFound = errors.New("book not found")

const (
	historyLimit         = 25
	checkpointPercentGap = 2.0
	checkpointTimeGap    = 10 * time.Minute
	finishedThreshold    = 98.0
)

type BookRepository struct {
	collection *mongo.Collection
}

func NewBookRepository(client *storage.Client) *BookRepository {
	return &BookRepository{collection: client.Collection("books")}
}

func (r *BookRepository) Register(ctx context.Context, owner string, req models.RegisterRequest) (models.Book, error) {
	now := time.Now().UTC()
	filter := bson.D{{Key: "owner", Value: owner}, {Key: "key", Value: req.Key}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "title", Value: req.Title},
			{Key: "author", Value: req.Author},
			{Key: "format", Value: req.Format},
			{Key: "sizeBytes", Value: req.SizeBytes},
			{Key: "updatedAt", Value: now},
		}},
		{Key: "$inc", Value: bson.D{{Key: "openCount", Value: 1}}},
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "owner", Value: owner},
			{Key: "key", Value: req.Key},
			{Key: "createdAt", Value: now},
			{Key: "finished", Value: false},
			{Key: "secondsRead", Value: int64(0)},
			{Key: "history", Value: []models.Location{}},
			{Key: "current", Value: models.Location{}},
		}},
	}

	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var book models.Book
	if err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&book); err != nil {
		return models.Book{}, err
	}
	return book, nil
}

func (r *BookRepository) Get(ctx context.Context, owner, key string) (models.Book, error) {
	filter := bson.D{{Key: "owner", Value: owner}, {Key: "key", Value: key}}

	var book models.Book
	err := r.collection.FindOne(ctx, filter).Decode(&book)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.Book{}, ErrNotFound
	}
	if err != nil {
		return models.Book{}, err
	}
	return book, nil
}

func (r *BookRepository) List(ctx context.Context, owner string) ([]models.Book, error) {
	opts := options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}})

	cursor, err := r.collection.Find(ctx, bson.D{{Key: "owner", Value: owner}}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	books := []models.Book{}
	if err := cursor.All(ctx, &books); err != nil {
		return nil, err
	}
	for i := range books {
		if books[i].History == nil {
			books[i].History = []models.Location{}
		}
	}
	return books, nil
}

func (r *BookRepository) SaveProgress(ctx context.Context, owner, key string, req models.ProgressRequest) (models.Book, error) {
	now := time.Now().UTC()
	location := models.Location{
		Page:     req.Page,
		Pages:    req.Pages,
		Cfi:      req.Cfi,
		Offset:   req.Offset,
		Percent:  clampPercent(req.Percent),
		Label:    req.Label,
		Recorded: now,
	}

	filter := bson.D{{Key: "owner", Value: owner}, {Key: "key", Value: key}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "current", Value: location},
			{Key: "updatedAt", Value: now},
			{Key: "finished", Value: location.Percent >= finishedThreshold},
		}},
		{Key: "$inc", Value: bson.D{{Key: "secondsRead", Value: normalizeSeconds(req.SecondsRead)}}},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.Before)

	var previous models.Book
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&previous)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.Book{}, ErrNotFound
	}
	if err != nil {
		return models.Book{}, err
	}

	if shouldCheckpoint(previous, location) {
		if err := r.appendHistory(ctx, filter, location); err != nil {
			return models.Book{}, err
		}
	}

	return r.Get(ctx, owner, key)
}

func (r *BookRepository) Reset(ctx context.Context, owner, key string) (models.Book, error) {
	filter := bson.D{{Key: "owner", Value: owner}, {Key: "key", Value: key}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "current", Value: models.Location{Recorded: time.Now().UTC()}},
			{Key: "finished", Value: false},
			{Key: "updatedAt", Value: time.Now().UTC()},
		}},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return models.Book{}, err
	}
	if result.MatchedCount == 0 {
		return models.Book{}, ErrNotFound
	}
	return r.Get(ctx, owner, key)
}

func (r *BookRepository) Restore(ctx context.Context, owner, key string, index int) (models.Book, error) {
	book, err := r.Get(ctx, owner, key)
	if err != nil {
		return models.Book{}, err
	}
	if index < 0 || index >= len(book.History) {
		return models.Book{}, ErrNotFound
	}

	target := book.History[index]
	target.Recorded = time.Now().UTC()

	filter := bson.D{{Key: "owner", Value: owner}, {Key: "key", Value: key}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "current", Value: target},
			{Key: "finished", Value: target.Percent >= finishedThreshold},
			{Key: "updatedAt", Value: time.Now().UTC()},
		}},
	}

	if _, err := r.collection.UpdateOne(ctx, filter, update); err != nil {
		return models.Book{}, err
	}
	return r.Get(ctx, owner, key)
}

func (r *BookRepository) Delete(ctx context.Context, owner, key string) error {
	filter := bson.D{{Key: "owner", Value: owner}, {Key: "key", Value: key}}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BookRepository) appendHistory(ctx context.Context, filter bson.D, location models.Location) error {
	update := bson.D{
		{Key: "$push", Value: bson.D{
			{Key: "history", Value: bson.D{
				{Key: "$each", Value: []models.Location{location}},
				{Key: "$slice", Value: -historyLimit},
			}},
		}},
	}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func shouldCheckpoint(previous models.Book, next models.Location) bool {
	if len(previous.History) == 0 {
		return next.Percent > 0
	}

	last := previous.History[len(previous.History)-1]
	if absDiff(last.Percent, next.Percent) >= checkpointPercentGap {
		return true
	}
	return time.Since(last.Recorded) >= checkpointTimeGap && next.Percent > 0
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizeSeconds(value int64) int64 {
	if value < 0 || value > 3600 {
		return 0
	}
	return value
}
