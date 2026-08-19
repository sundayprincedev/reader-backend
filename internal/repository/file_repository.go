package repository

import (
	"context"
	"errors"
	"io"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/sundayprincedev/reader-backend/internal/storage"
)

type FileRepository struct {
	bucket *mongo.GridFSBucket
}

func NewFileRepository(client *storage.Client) *FileRepository {
	return &FileRepository{bucket: client.Bucket()}
}

func (r *FileRepository) Store(ctx context.Context, name string, source io.Reader) (bson.ObjectID, error) {
	return r.bucket.UploadFromStream(ctx, name, source)
}

func (r *FileRepository) Stream(ctx context.Context, id bson.ObjectID, target io.Writer) error {
	_, err := r.bucket.DownloadToStream(ctx, id, target)
	if errors.Is(err, mongo.ErrFileNotFound) {
		return ErrNotFound
	}
	return err
}

func (r *FileRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	err := r.bucket.Delete(ctx, id)
	if errors.Is(err, mongo.ErrFileNotFound) {
		return nil
	}
	return err
}
