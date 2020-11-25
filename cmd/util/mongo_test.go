package util

import (
	"context"
	"log"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func Teardown(client *mongo.Client, mp MongoParams) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Database(mp.Database).Drop(ctx); err != nil {
		log.Fatalf("Teardown - Failed to drop testing database: %v", err)
	}

}

func TestConnectToMongo(t *testing.T) {
	// Load env vars
	mp := LoadMongoParams()
	client := ConnectToMongo(mp.URI, mp.Database, mp.Collection)
	defer Teardown(client, mp)
}

// TestWriteOneToMongo inserts a single document
func TestWriteOneToMongo(t *testing.T) {
	mp := LoadMongoParams()
	client := ConnectToMongo(mp.URI, mp.Database, mp.Collection)
	defer Teardown(client, mp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.Database(mp.Database).Collection(mp.Collection).InsertOne(ctx, bson.D{
		{"key1", "value1"},
		{"key2", "value2"},
	})
	if err != nil {
		t.Errorf("Failed - TestWriteOneToMongo - %v", err)
	}
}
