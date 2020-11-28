package main

import (
	"github.com/mileshill/serialreader/cmd/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"os"
	"time"
)

// Record is for decoding the mongo queries
type Record struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Timestamp int                `json:"timestamp"`
	Device    string             `json:"device"`
	Payload   string             `json:"payload"`
}

// RecordForPayload encodes for the HTTP POST to API
type RecordForPayload struct {
	Timestamp int    `json:"timestamp"`
	Device    string `json:"device"`
	Payload   string `json:"payload"`
}

// RequestPayload is the full body to be JSON marshalled for API consumption
type RequestPayload struct {
	Timestamp int                `json:"timestamp"`
	Device    string             `json:"device"`
	Data      []RecordForPayload `json:"data"`
}

// WorkerUpdateRecords is a background process for updating the records in the database.
// Pushing the workload off the main thread ensures data are consumed quickly and updates
// of those data are non-blocking
func WorkerUpdateRecords(ch <-chan bson.A, client *mongo.Client, mp util.MongoParams) {
	for synced := range ch {
		util.UpdateSyncStatus(client, mp, synced)
	}
}

// WorkerDeleteRecords periodically removes any synced records from the database. This allows the
// database to remain lightweight and the query performance to be maintained
func WorkerDeleteRecords(client *mongo.Client, mp util.MongoParams) {
	for {
		util.DeleteSynced(client, mp)
		time.Sleep(60 * time.Second)
	}
}

// main
func main() {

	// Connect to Mongo
	mp := util.LoadMongoParams()
	client := util.ConnectToMongo(mp.ContextConnect, mp.URI, mp.Database, mp.Collection)
	defer client.Disconnect(mp.ContextRequest)

	// Start background process to update records
	chBSON := make(chan bson.A, 10)
	for i := 1; i <= 3; i++ {
		go WorkerUpdateRecords(chBSON, client, mp)
	}

	// Start background process to delete records
	go WorkerDeleteRecords(client, mp)

	// Host info
	device, err := os.Hostname()
	if err != nil {
		log.Fatalf("main - Could not get hostname - %v", err)
	}

	// Get oldest non-synced records
	batchSize := 25
	for {
		cursor := util.GetNextBatch(client, mp, batchSize)

		// If no data are being returned, sleep and check again
		if cursor.RemainingBatchLength() == 0 {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Record ids for sync check
		var records []Record
		err := cursor.All(mp.ContextConnect, &records)
		if err != nil {
			log.Fatalf("Cursor decode failed - %v", err)
		}
		// Records for payload
		var recordsPayload []RecordForPayload
		for _, rec := range records {
			recordsPayload = append(recordsPayload, RecordForPayload{
				Timestamp: rec.Timestamp,
				Device:    device,
				Payload:   rec.Payload,
			})
		}
		log.Printf("Synced %d to API", len(records))

		// Build payload from cursor to send to API
		//payload := &RequestPayload{
		//	Timestamp: int(time.Now().Unix()),
		//	Device:    device,
		//	Data:      recordsPayload,
		//}
		//payloadBuffer, err := json.Marshal(payload)
		//if err != nil {
		//	log.Fatalf("Failed to marshal to json - %v", err)
		//}


		// Array of ids to update
		var syncedIds bson.A
		for _, rec := range records {
			id, err := primitive.ObjectIDFromHex(rec.ID.Hex())
			if err != nil {
				log.Fatalf("Failed to parse Id - %v", err)
			}
			syncedIds = append(syncedIds, id)
		}
		chBSON<- syncedIds

	}

}
