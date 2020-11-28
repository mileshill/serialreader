# serialreader


## Running
### docker-compose
Start all the services; reader, producer and mongodb.
```shell script
cd /path/to/serialreader
docker-compose up -d
```

Just mongodb
```
docker run -d -p 27017:27017 --network="host" mongo
```

Just the reader
```shell script
docker build -t reader -f reader.dockerfile .
docker run --device /dev/ttyS0 --network="host" reader
```

Just the producer
```shell script
docker build -t producer -f producer.dockerfile .
docker run --network="host" producer
```
### 