package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DraftRepository interface {
	GetDraft(ctx context.Context, projectID string, version int) (bson.M, error)
	InsertDraft(ctx context.Context, projectID string, ownerID string, draft []bson.M, version int) error

	GetDraftOwner(ctx context.Context, projectID string) (string, error)
	GetLatestMutationForID(ctx context.Context, projectID string, elementID string) (bson.M, error)
	GetLatestMutationVersion(ctx context.Context, projectID string) (int, error)
	GetMutationsInRange(ctx context.Context, projectID string, from int, to int) (*[]bson.M, error)
	InsertMutation(ctx context.Context, projectID string, ownerID string, mutation bson.M) (int, error)

	Close(ctx context.Context) error
}

type draftRepository struct {
	draftsCollection    *mongo.Collection
	mutationsCollection *mongo.Collection
	client              *mongo.Client
}

func NewDraftRepository(uri string, dbName string, ttlDays int) (DraftRepository, error) {
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
		client.Disconnect(ctx)
		return nil, fmt.Errorf("mongo ping error: %w", err)
	}

	db := client.Database(dbName)

	repo := &draftRepository{
		draftsCollection:    db.Collection("drafts"),
		mutationsCollection: db.Collection("mutations"),
		client:              client,
	}

	if err = createIndexes(repo.draftsCollection, repo.mutationsCollection, ttlDays); err != nil {
		return nil, fmt.Errorf("mongo index create error: %w", err)
	}

	log.Println("Connected to MongoDB")
	return repo, nil
}

func createIndexes(draftsCollection, mutationsCollection *mongo.Collection, ttlDays int) error {
	_, err := draftsCollection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "project_id", Value: 1}, {Key: "version", Value: 1}},
	})
	if err != nil {
		return err
	}

	_, err = draftsCollection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(int32(ttlDays * 24 * 3600)),
	})
	if err != nil {
		return err
	}

	_, err = mutationsCollection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "project_id", Value: 1}, {Key: "version", Value: 1}},
	})
	if err != nil {
		return err
	}

	_, err = mutationsCollection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "project_id", Value: 1}, {Key: "id", Value: 1}, {Key: "created_at", Value: -1}},
	})
	if err != nil {
		return err
	}

	_, err = mutationsCollection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(int32(ttlDays * 24 * 3600)),
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *draftRepository) Close(ctx context.Context) error {
	return r.client.Disconnect(ctx)
}

func (r *draftRepository) GetDraft(ctx context.Context, projectID string, version int) (bson.M, error) {
	filter := bson.M{
		"project_id": projectID,
		"version":    bson.M{"$lte": version},
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})

	var draft bson.M
	err := r.draftsCollection.FindOne(ctx, filter, opts).Decode(&draft)
	if err == mongo.ErrNoDocuments {
		return bson.M{"version": 0, "mutations": bson.A{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find draft error: %w", err)
	}
	return draft, nil
}

func (r *draftRepository) InsertDraft(ctx context.Context, projectID string, ownerID string, mutations []bson.M, version int) error {
	draft := bson.M{
		"project_id": projectID,
		"owner_id":   ownerID,
		"version":    version,
		"created_at": time.Now(),
		"mutations":  mutations,
	}

	_, err := r.draftsCollection.InsertOne(ctx, draft)
	if err != nil {
		return err
	}

	return nil
}

func (r *draftRepository) GetDraftOwner(ctx context.Context, projectID string) (string, error) {
	filter := bson.M{
		"project_id": projectID,
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: 1}})
	var result struct {
		OwnerID string `bson:"owner_id"`
	}
	err := r.mutationsCollection.FindOne(ctx, filter, opts).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return result.OwnerID, nil
}

func (r *draftRepository) GetLatestMutationForID(ctx context.Context, projectID string, elementID string) (bson.M, error) {
	filter := bson.M{
		"project_id": projectID,
		"id":         elementID,
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})
	var mutation bson.M
	err := r.mutationsCollection.FindOne(ctx, filter, opts).Decode(&mutation)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mutation, nil
}

func (r *draftRepository) GetLatestMutationVersion(ctx context.Context, projectID string) (int, error) {
	filter := bson.M{"project_id": projectID}
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})

	var result struct {
		Version int `bson:"version"`
	}

	err := r.mutationsCollection.FindOne(ctx, filter, opts).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("find latest mutation error: %w", err)
	}

	return result.Version, nil
}

func (r *draftRepository) GetMutationsInRange(ctx context.Context, projectID string, from int, to int) (*[]bson.M, error) {
	filter := bson.M{
		"project_id": projectID,
		"version": bson.M{
			"$gte": from,
			"$lte": to,
		},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "version", Value: 1}}).
		SetLimit(int64(to - from + 1))

	cursor, err := r.mutationsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("find mutations in range error: %w", err)
	}
	defer cursor.Close(ctx)

	var mutations []bson.M
	if err = cursor.All(ctx, &mutations); err != nil {
		return nil, fmt.Errorf("decode mutations error: %w", err)
	}

	return &mutations, nil
}

func (r *draftRepository) InsertMutation(ctx context.Context, projectID string, ownerID string, mutation bson.M) (int, error) {
	latestVersion, err := r.GetLatestMutationVersion(ctx, projectID)
	if err != nil {
		return 0, err
	}

	mutation["project_id"] = projectID
	mutation["owner_id"] = ownerID
	mutation["version"] = latestVersion + 1
	mutation["created_at"] = time.Now()

	_, err = r.mutationsCollection.InsertOne(ctx, mutation)
	if err != nil {
		return 0, err
	}

	return latestVersion + 1, nil
}
