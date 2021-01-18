package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"github.com/mileshill/serialreader/cmd/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Record is for decoding the mongo queries
type Record struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	Timestamp  int                `json:"timestamp"`
	Payload    string             `json:"payload"`
	SerialPort string             `json:"serialPort"`
}

// RecordForPayload encodes for the HTTP POST to API
type RecordForPayload struct {
	Timestamp  int    `json:"timestamp_utc_recorded"`
	Payload    string `json:"payload"`
	SerialPort string `json:"serialPort"`
}

// RequestPayload is the full body to be JSON marshalled for API consumption
type RequestPayload struct {
	Timestamp int                `json:"timestamp_utc_transmitted"`
	Hostname  string             `json:"hostname"`
	Data      []RecordForPayload `json:"data"`
}

type PingServerPayload struct {
	Hostname string `json:"hostname"`
	Timestamp int `json:"timestamp_utc"`
	LastRecordTimestamp int `json:"timestamp_utc_recorded_last"`
	Delta int `json:"delta_ping_last_record_seconds"`
}

// lastRecordTime
var lastRecordTime int

// WorkerUpdateRecords is a background process for updating the records in the database.
// Pushing the workload off the main thread ensures data are consumed quickly and updates
// of those data are non-blocking
func WorkerUpdateRecords(ch <-chan bson.A, client *mongo.Client, mp util.MongoParams) {
	for synced := range ch {
		util.UpdateSyncStatus(client, mp, synced)
	}
}

// Worker
func WorkerPingServer(){
	url := os.Getenv("API_URL_PING")
	if url == "" {
		log.Printf("main.WorkerPingServer - ENV=API_URL_PING not found. Restart once var is set")
		return
	}
	device := util.GetEnvWithFallback("HOSTNAME", "")
	if device == "" {
		log.Fatalf("main - HOSTNAME not set as env var")
	}
	for {
		time.Sleep(60 * time.Second)
		currentTime := int(time.Now().Unix())
		pingPayload := &PingServerPayload{
			Hostname: device,
			Timestamp: currentTime,
			LastRecordTimestamp: lastRecordTime,
			Delta: currentTime - lastRecordTime,
		}

		payloadMarshal, err := json.Marshal(pingPayload)
		if err != nil {
			log.Fatalf("ERROR: main.WorkerPingServer - %v", err)
		}
		// Make request here
		var payloadBuffer bytes.Buffer
		gz := gzip.NewWriter(&payloadBuffer)
		numBytes, err := gz.Write(payloadMarshal)
		if err != nil {
			log.Fatalf("Failed to zip marshalled json - %v", err)
		}
		err = gz.Close()
		if err != nil {
			log.Fatalf("Failed to close zipped payload - %v", err)
		}

		req, err := http.NewRequest("POST", url, &payloadBuffer)
		if err != nil {
			log.Fatalf("ERROR: main.WorkerPingServer - Failed to create new POST request object - %v", err)
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Length", strconv.Itoa(numBytes))

		httpClient := &http.Client{}
		resp, err := httpClient.Do(req)

		if err != nil {
			log.Fatalf("Error POST to API %s - %v", url, err)
		}
		if resp.StatusCode != 201 {
			var body []byte
			_, err := resp.Body.Read(body)
			if err != nil {
				log.Fatalf("Failed to decode API response with Code %d - %v", resp.StatusCode, err)
			}
			log.Fatalf("Failed to write PING to API - %s", string(body))
		}
		if resp.StatusCode == 201 {
			log.Printf("main.WorkerPingServer - Successfully pinged server")
		}
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

func setMaxRecordTime(records []RecordForPayload) {
	max := 0
	for _, rec := range records{
		if rec.Timestamp > max {
			max = rec.Timestamp
		}
	}
	lastRecordTime = max  // Updates the global var
}


// main
func main() {
	// Connect to Mongo
	mp := util.LoadMongoParams()
	client := util.ConnectToMongo(mp.ContextConnect, mp.URI, mp.Database, mp.Collection)
	defer client.Disconnect(mp.ContextRequest)

	// Start background process to delete records
	go WorkerDeleteRecords(client, mp)
	go WorkerPingServer()

	// Host info
	device := util.GetEnvWithFallback("HOSTNAME", "")
	if device == "" {
		log.Fatalf("main - HOSTNAME not set as env var")
	}

	// Get oldest non-synced records
	apiUrl := util.GetEnvWithFallback("API_ENDPOINT", "") // Base end point
	if apiUrl == "" {
		log.Fatalf("Error - `API` not set in environment")
	}
	httpClient := &http.Client{}
	batchSize := 25 // Max size allowed by Dynamodb
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
				Timestamp:  rec.Timestamp,
				Payload:    rec.Payload,
				SerialPort: rec.SerialPort,
			})
		}
		go setMaxRecordTime(recordsPayload) // Update in the background
		// Build payload from cursor to send to API
		payload := &RequestPayload{
			Timestamp: int(time.Now().Unix()),
			Hostname:  device,
			Data:      removeDuplicateRecords(recordsPayload),
		}
		payloadMarshal, err := json.Marshal(payload)
		if err != nil {
			log.Fatalf("Failed to marshal to json - %v", err)
		}
		var payloadBuffer bytes.Buffer
		gz := gzip.NewWriter(&payloadBuffer)
		numBytes, err := gz.Write(payloadMarshal)
		if err != nil {
			log.Fatalf("Failed to zip marshalled json - %v", err)
		}
		err = gz.Close()
		if err != nil {
			log.Fatalf("Failed to close zipped payload - %v", err)
		}

		//resp, err := http.Post(apiUrl, "application/json", bytes.NewBuffer(payloadMarshal))
		req, err := http.NewRequest("POST", apiUrl, &payloadBuffer)
		if err != nil {
			log.Fatalf("Failed to create new POST request object - %v", err)
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Length", strconv.Itoa(numBytes))
		resp, err := httpClient.Do(req)

		//resp, err := http.Post(apiUrl, "application/json", bytes.NewBuffer(payloadMarshal))
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
		util.UpdateSyncStatus(client, mp, syncedIds)
	}

}
