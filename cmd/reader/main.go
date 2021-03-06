package main

import (
	"context"
	"github.com/mileshill/serialreader/cmd/util"
	"go.mongodb.org/mongo-driver/bson"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jacobsa/go-serial/serial"
)

// Struct to hold time elements
type CurrentTime struct {
	timestamp     int64
	humanReadable string
}

// currentUTCTime returns the number of seconds since
func currentUTCTime() CurrentTime {
	current := time.Now()
	return CurrentTime{
		timestamp:     current.Unix(),
		humanReadable: current.Format("%Y-%m-%dT%H:%M:%SZ"),
	}
}

// connectToPort
func connectToPort(portName string, baudRate uint) io.ReadWriteCloser {
	log.Printf("main.connectToPort - START - PORT %s BAUDRATE %d", portName, baudRate)

	standardBaudRate := serial.IsStandardBaudRate(baudRate)
	if !standardBaudRate {
		log.Printf("main.connectToPort - BaudRate %d is nonstandard. This may cause issues!", baudRate)
	}

	// Set up options
	options := serial.OpenOptions{
		PortName:              portName,
		BaudRate:              baudRate,
		DataBits:              8,
		StopBits:              1,
		MinimumReadSize:       4,
		InterCharacterTimeout: uint(1000),
	}

	// Open the port
	port, err := serial.Open(options)
	if err != nil {
		log.Panicf("main.serial.Open: %v", err)
	}
	log.Printf("main.connectToPort - COMPLETE")
	return port
}

func formatPayloadForDatabase(payload string) string {
	// Remove NULL chars
	modified := strings.Replace(payload, "\u0000", ",", -1)
	// Remove carriage returns
	re := regexp.MustCompile(`\r?\n`)
	return re.ReplaceAllString(modified, "")
}

func main() {
	// Serial Port connection
	portName := util.GetEnvWithFallback("SERIAL_PORT", "/dev/ttyS0")
	baudRate, err := strconv.ParseUint(util.GetEnvWithFallback("SERIAL_BAUDRATE", "9600"), 10, 8)
	if err != nil {
		log.Printf("main - Parse BaudRate from env. Using default of 9600")
		baudRate = 9600
	}
	port := connectToPort(portName, uint(baudRate))
	defer port.Close()

	// Database connection
	mp := util.LoadMongoParams()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	client := util.ConnectToMongo(ctx, mp.URI, mp.Database, mp.Collection)
	defer client.Disconnect(ctx)
	defer cancel()

	log.Printf("main - Starting serial read loop")
	delimiter := util.GetEnvWithFallback("SERIAL_DELIMITER", "\n")
	log.Printf("main - SERAIL DELIMITER - %s", delimiter)
	loopStartTime := currentUTCTime()
	// Outer Loop - Handles timeouts from the serial reader and insert into database
	for {
		var data strings.Builder // Build record
		buff := make([]byte, 1)  // Buffer to hold the data
		// Inner Loop - Reads bytes until terminating character is received
		for {
			n, _ := port.Read(buff)
			if n == 0 {
				break
			}

			if string(buff) == delimiter {
				break
			}
			for _, element := range buff {
				data.WriteByte(element)
			}

		}
		// currentTime will be the time of the data insert it. It serves as the watermark for data ingestion
		currentTime := currentUTCTime()

		// Validate data exists and within last two minutes
		if (currentTime.timestamp - loopStartTime.timestamp) >= 120 {
			log.Panicf("main - Loop failed to read serial data after 2 minutes. Shutting reader down")
		}

		// Prep the data
		record := bson.D{
			{"timestamp", int(currentTime.timestamp)},
			{"payload", formatPayloadForDatabase(data.String())},
			{"synced", false},
			{"serialPort", portName},
		}
		log.Printf("main - record.Payload - %s", record[1])

		// Insert
		insertResult, err := client.Database(mp.Database).Collection(mp.Collection).InsertOne(ctx, record)
		if err != nil {
			log.Printf("%s", insertResult)
			log.Panicf("main - Reader Loop - Failed to Insert: %v", err)
		}
		log.Printf("Insert result id: %s", insertResult.InsertedID)

		// Update loop start time to ensure timeout failures occur only after
		// two minutes of no new data
		loopStartTime = currentTime
	}
}
