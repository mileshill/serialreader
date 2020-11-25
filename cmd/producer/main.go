package main

import (
	"encoding/json"
	"github.com/mileshill/serialreader/cmd/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"os"
	"time"
)

// Used to insert into local database
type Record struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Timestamp int                `json:"timestamp"`
	Device    string             `json:"device"`
	Payload   string             `json:"payload"`
}

type RecordForPayload struct {
	Timestamp int    `json:"timestamp"`
	Device    string `json:"device"`
	Payload   string `json:"payload"`
}

// Request sent to API
type RequestPayload struct {
	Timestamp int                `json:"timestamp"`
	Device    string             `json:"device"`
	Data      []RecordForPayload `json:"data"`
}

func main() {
	// Connect to Mongo
	mp := util.LoadMongoParams()
	client := util.ConnectToMongo(mp.ContextConnect, mp.URI, mp.Database, mp.Collection)
	defer client.Disconnect(mp.ContextRequest)

	// Host info
	device, err := os.Hostname()
	if err != nil {
		log.Fatalf("main - Could not get hostname - %v", err)
	}

	// Get oldest unsycned records
	batchSize := 10
	for {
		cursor := util.GetNextBatch(client, mp, batchSize)
		if cursor.RemainingBatchLength() == 0 {
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

		// Build payload from cursor to send to API
		payload := &RequestPayload{
			Timestamp: int(time.Now().Unix()),
			Device:    device,
			Data:      recordsPayload,
		}
		payloadBuffer, err := json.Marshal(payload)
		if err != nil {
			log.Fatalf("Failed to marshal to json - %v", err)
		}

		// Send to api
		// Pass records to workers for deletion if succesful request

		// Array of ids to update
		//syncedIds := make([]string, len(records))
		var syncedIds bson.A
		for _, rec := range records {
			id, err := primitive.ObjectIDFromHex(rec.ID.Hex())
			if err != nil {
				log.Fatalf("Failed to parse Id - %v", err)
			}
			syncedIds = append(syncedIds, id)
		}

		util.UpdateSyncStatus(client, mp, syncedIds)
		util.DeleteSynced(client, mp)
	}

}
