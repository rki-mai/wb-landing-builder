package publishing

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PublicationRepository хранит метаданные публикаций в MongoDB.
type PublicationRepository interface {
	Insert(ctx context.Context, pub Publication) error
	Get(ctx context.Context, id string) (*Publication, error)
	Delete(ctx context.Context, id string) error
	Close(ctx context.Context) error
}

type publicationRepository struct {
	collection *mongo.Collection
	client     *mongo.Client
}

// NewPublicationRepository подключается к MongoDB и создаёт коллекцию publications.
func NewPublicationRepository(uri, dbName string, ttlDays int) (PublicationRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx,
		options.Client().
			ApplyURI(uri).
			SetRetryWrites(true).
			SetRetryReads(true))
	if err != nil {
		return nil, fmt.Errorf("mongo connect error: %w", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("mongo ping error: %w", err)
	}

	collection := client.Database(dbName).Collection("publications")
	repo := &publicationRepository{
		collection: collection,
		client:     client,
	}

	if err = createPublicationIndexes(collection, ttlDays); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("publication index create error: %w", err)
	}

	log.Println("Connected to MongoDB (publications)")
	return repo, nil
}

func createPublicationIndexes(collection *mongo.Collection, ttlDays int) error {
	ctx := context.Background()

	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "project_id", Value: 1}},
	})
	if err != nil {
		return err
	}

	_, err = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(int32(ttlDays * 24 * 3600)),
	})
	return err
}

func (r *publicationRepository) Insert(ctx context.Context, pub Publication) error {
	_, err := r.collection.InsertOne(ctx, pub)
	if err != nil {
		return fmt.Errorf("insert publication: %w", err)
	}
	return nil
}

func (r *publicationRepository) Get(ctx context.Context, id string) (*Publication, error) {
	var pub Publication
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&pub)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find publication: %w", err)
	}
	return &pub, nil
}

func (r *publicationRepository) Delete(ctx context.Context, id string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete publication: %w", err)
	}
	return nil
}

func (r *publicationRepository) Close(ctx context.Context) error {
	return r.client.Disconnect(ctx)
}
