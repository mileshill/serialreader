package util

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
)

// ConnectToDatabase
func ConnectToMongo(ctx context.Context, uri string, database string, collection string) *mongo.Client {
	log.Printf("main.ConnectToMongo - START: URI %s DATABASE %s COLLECTION %s", uri, database, collection)
	// New client
	client, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("main.ConnectToMongo.NewClient: %v", err)
	}
	// Make connection

	err = client.Connect(ctx)
	if err != nil {
		log.Fatalf("main.ConnectToMongo.Connect: %v", err)
	}

	// Check connection
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Fatalf("main.ConnectToMongo.Ping: %v", err)
	}

	log.Printf("main.ConnectToMongo - COMPLETE")
	return client
}
