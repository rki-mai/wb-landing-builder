package repository

import (
	"context"
	"fmt"
	"time"
	"wb-landing-builder/auth/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AuthRepository interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id string) (*models.User, error)

	SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error
	GetRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error

	Close(ctx context.Context) error
}

type authRepository struct {
	usersCollection         *mongo.Collection
	refreshTokensCollection *mongo.Collection
	client                  *mongo.Client
}

func NewAuthRepository(uri string, dbName string) (AuthRepository, error) {
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

	repo := &authRepository{
		usersCollection:         db.Collection("users"),
		refreshTokensCollection: db.Collection("refresh_tokens"),
		client:                  client,
	}

	if err := createIndexes(repo); err != nil {
		return nil, err
	}

	return repo, nil
}

func createIndexes(r *authRepository) error {
	_, err := r.usersCollection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	_, err = r.refreshTokensCollection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "token", Value: 1}},
	})
	if err != nil {
		return err
	}

	_, err = r.refreshTokensCollection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *authRepository) CreateUser(ctx context.Context, user *models.User) error {
	_, err := r.usersCollection.InsertOne(ctx, user)
	return err
}

func (r *authRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User

	err := r.usersCollection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *authRepository) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User

	err := r.usersCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *authRepository) SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	_, err := r.refreshTokensCollection.InsertOne(ctx, token)
	return err
}

func (r *authRepository) GetRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	var rt models.RefreshToken

	err := r.refreshTokensCollection.FindOne(ctx, bson.M{"token": token}).Decode(&rt)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &rt, nil
}

func (r *authRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	_, err := r.refreshTokensCollection.DeleteOne(ctx, bson.M{"token": token})
	return err
}

func (r *authRepository) Close(ctx context.Context) error {
	return r.client.Disconnect(ctx)
}
