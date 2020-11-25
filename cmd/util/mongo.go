package util

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
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

// GetNextBatch
func GetNextBatch(client *mongo.Client, mp MongoParams, batchSize int) *mongo.Cursor {
	// Configure query options
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"timestamp", 1}}) // Ascending sort on time; oldest first!
	findOptions.SetBatchSize(int32(batchSize))
	findOptions.SetLimit(int64(batchSize))

	// Reference the collection
	col := client.Database(mp.Database).Collection(mp.Collection)

	// Create filter
	filter := bson.D{
		{"synced", false},
	}

	// Execute query
	cursor, err := col.Find(mp.ContextConnect, filter, findOptions)
	if err != nil {
		log.Fatalf("mongo.GetNextBatch - %v", err)
	}
	return cursor
}

// UpdateSyncStatus updates the `synced` field on records to `true` once they have been consumed by the
// API and a response.status == 200 is returned
func UpdateSyncStatus(client *mongo.Client, mp MongoParams, synced bson.A) {
	col := client.Database(mp.Database).Collection(mp.Collection)
	filter := bson.D{{"_id", bson.D{{"$in", synced}}}} // Search by array of _id

	updates := bson.D{{"$set", bson.D{{"synced", true}}}} // Update sync status
	result, err := col.UpdateMany(mp.ContextConnect, filter, updates)
	if err != nil {
		log.Fatalf("mongo.UpdateSyncStatus - %v", err)
	}
	log.Printf("mongo.UpdateSyncStatus - Matched %d, Updated %d ", result.MatchedCount, result.ModifiedCount)
}

// DeleteSynced clears the database of any records already sent to the API. This keeps memory usage down
// and the queries operting on the smallest dataset possible
func DeleteSynced(client *mongo.Client, mp MongoParams) {
	col := client.Database(mp.Database).Collection(mp.Collection)
	filter := bson.D{{"synced", true}}
	result, err := col.DeleteMany(mp.ContextConnect, filter)
	if err != nil {
		log.Fatalf("mongo.DeleteSynced - %v", err)
	}
	log.Printf("mongo.DeleteSynced - %d", result.DeletedCount)
}
