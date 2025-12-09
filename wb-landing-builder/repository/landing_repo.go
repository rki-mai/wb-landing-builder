// repository/landing_repo.go
package repository

import (
	"context"
	"log"
	"wb-landing-builder/db"
	"wb-landing-builder/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func getCollection() *mongo.Collection {
	return db.Client.Database("landing_builder").Collection("landings")
}

func CreateLanding(landing *models.Landing) (primitive.ObjectID, error) {
	landing.ID = primitive.NilObjectID
	res, err := getCollection().InsertOne(context.TODO(), landing)
	if err != nil {
		log.Printf("Ошибка сохранения лендинга: %v", err)
		return primitive.NilObjectID, err
	}
	insertedID := res.InsertedID.(primitive.ObjectID)
	log.Printf("Создан лендинг с ID: %v", insertedID)
	return insertedID, nil
}

func GetLandingByID(id string) (*models.Landing, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var landing models.Landing
	err = getCollection().FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&landing)
	return &landing, err
}

func UpdateLanding(id string, landing *models.Landing) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = getCollection().ReplaceOne(context.TODO(), bson.M{"_id": objID}, landing)
	return err
}

func GetPublishedLandings() (*mongo.Cursor, error) {
	return getCollection().Find(context.TODO(), bson.M{"published": true})
}

func GetAllLandings() (*mongo.Cursor, error) {
	return getCollection().Find(context.TODO(), bson.M{}) // пустой фильтр = все документы
}

func DeleteLandingByID(id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = getCollection().DeleteOne(context.TODO(), bson.M{"_id": objID})
	return err
}
