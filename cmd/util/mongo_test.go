package util

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"testing"
)

func Teardown(client *mongo.Client, mp MongoParams) {
	if err := client.Database(mp.Database).Drop(mp.ContextRequest); err != nil {
		log.Fatalf("Teardown - Failed to drop testing database: %v", err)
	}
}

func TestConnectToMongo(t *testing.T) {
	// Load env vars
	mp := LoadMongoParams()
	client := ConnectToMongo(mp.ContextConnect, mp.URI, mp.Database, mp.Collection)
	defer Teardown(client, mp)
}

// TestWriteOneToMongo inserts a single document
func TestWriteOneToMongo(t *testing.T) {
	mp := LoadMongoParams()
	client := ConnectToMongo(mp.ContextConnect, mp.URI, mp.Database, mp.Collection)
	defer Teardown(client, mp)

	_, err := client.Database(mp.Database).Collection(mp.Collection).InsertOne(mp.ContextRequest, bson.D{
		{"key1", "value1"},
		{"key2", "value2"},
	})
	if err != nil {
		t.Errorf("Failed - TestWriteOneToMongo - %v", err)
	}
}
