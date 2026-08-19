package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/sundayprincedev/reader-backend/internal/models"
	"github.com/sundayprincedev/reader-backend/internal/storage"
)

var ErrEmailTaken = errors.New("email already registered")

type UserRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(client *storage.Client) *UserRepository {
	return &UserRepository{collection: client.Collection("users")}
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash string) (models.User, error) {
	user := models.User{
		Email:        NormalizeEmail(email),
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC(),
	}

	result, err := r.collection.InsertOne(ctx, user)
	if mongo.IsDuplicateKeyError(err) {
		return models.User{}, ErrEmailTaken
	}
	if err != nil {
		return models.User{}, err
	}

	id, ok := result.InsertedID.(bson.ObjectID)
	if !ok {
		return models.User{}, errors.New("unexpected identifier type")
	}

	user.ID = id
	return user, nil
}

func (r *UserRepository) ByEmail(ctx context.Context, email string) (models.User, error) {
	filter := bson.D{{Key: "email", Value: NormalizeEmail(email)}}

	var user models.User
	err := r.collection.FindOne(ctx, filter).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (r *UserRepository) ByID(ctx context.Context, id string) (models.User, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return models.User{}, ErrNotFound
	}

	var user models.User
	err = r.collection.FindOne(ctx, bson.D{{Key: "_id", Value: objectID}}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}
