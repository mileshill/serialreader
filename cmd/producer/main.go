package main

import (
	"bytes"
	"encoding/json"
	"github.com/mileshill/serialreader/cmd/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
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
	Timestamp int    `json:"timestamp_utc_recorded"`
	Hostname string `json:"hostname"`
	Payload   string `json:"payload"`
}

// RequestPayload is the full body to be JSON marshalled for API consumption
type RequestPayload struct {
	Timestamp int                `json:"timestamp_utc_transmitted"`
	Hostname    string             `json:"hostname"`
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

//
func removeDuplicateRecords(records []RecordForPayload) []RecordForPayload {
	keys := make(map[int]bool)
	var list []RecordForPayload
	for _, entry := range records {
		if _, value := keys[entry.Timestamp]; !value {
			keys[entry.Timestamp] = true
			list = append(list, entry)
		}
	}
	return list
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
	device := util.GetEnvWithFallback("HOSTNAME", "")
	if device == "" {
		log.Fatalf("main - HOSTNAME not set as env var")
	}

	// Get oldest non-synced records
	apiUrl := util.GetEnvWithFallback("API_ENDPOINT", "")
	if apiUrl == "" {
		log.Fatalf("Error - `API` not set in environment")
	}

	batchSize := 25  // Max size allowed by Dynamodb
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
				Hostname: device,
				Payload:   rec.Payload,
			})
		}

		// Build payload from cursor to send to API
		payload := &RequestPayload{
			Timestamp: int(time.Now().Unix()),
			Hostname:    device,
			Data:      removeDuplicateRecords(recordsPayload),
		}
		payloadMarshal, err := json.Marshal(payload)

		if err != nil {
			log.Fatalf("Failed to marshal to json - %v", err)
		}

		resp, err := http.Post(apiUrl, "application/json", bytes.NewBuffer(payloadMarshal))
		if err != nil {
			log.Fatalf("Error POST to API  - %v", err)
		}
		if resp.StatusCode != 201 {
			var body []byte
			_, err := resp.Body.Read(body)
			if err != nil {
				log.Fatalf("Failed to decode API response with Code %d - %v", resp.StatusCode, err)
			}
			log.Fatalf("Failed to write to API - %s", string(body))
		}
		if resp.StatusCode == 201 {
			log.Printf("Synced %d records to API", len(payload.Data))
		}


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
