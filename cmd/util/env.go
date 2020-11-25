package util

import "os"

func GetEnvWithFallback(key string, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

type MongoParams struct {
	URI        string
	Database   string
	Collection string
}

func LoadMongoParams() MongoParams {
	uri := GetEnvWithFallback("MONGO_URI", "mongodb://localhost:27017/test?retryWrites=true&w=majority")
	database := GetEnvWithFallback("MONGO_DATABASE", "test")
	collection := GetEnvWithFallback("MONGO_COLLECTION", "test")
	return MongoParams{
		URI:        uri,
		Database:   database,
		Collection: collection,
	}
}
