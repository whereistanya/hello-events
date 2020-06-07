package main

import (
	"fmt"
	"log"
	"time"

	"github.com/Shopify/sarama"
)

func main() {

	/***** Run a Kafka broker. *****/
	// To try this out, run a zookeeper, run a kafka on port 9092, create a Kafka
	// topic called 'burritos'. As of June 7, 2020, commands to do this were:
	// (starting from the root directory of a newly unzipped kafka tarball)
	// bin/zookeeper-server-start.sh config/zookeeper.properties
	// bin/kafka-server-start.sh config/server.properties
	// bin/kafka-topics.sh --create --zookeeper localhost:2181 --replication-factor 1 --partitions 2 --topic burritos

	brokers := []string{"localhost:9092"}
	topic := "burritos" // the most interesting topic

	/**** Create a config. *****/
	config := sarama.NewConfig()
	config.ClientID = "IllustrativeKafkaClient"
	// By default errors are logged and not returned. With this setting, we can
	// read from the Errors() channel as well as the Messages() channel.
	config.Consumer.Return.Errors = true

	// Required config option for a SyncProducer. This is how we know that we
	// successfully wrote. Will deadlock if we don't read from the Successes()
	// channel.
	config.Producer.Return.Successes = true

	// Choose a random partition to write to each time. In real life we'd maybe do
	// something deliberate with sharding on an attribute of the message.
	config.Producer.Partitioner = sarama.NewRandomPartitioner
	// All in-sync replicas must acknowledge receipt before we continue. Kafka has
	// a configuration for how we define in-sync, but by default it's everything
	// that has caught up with the leader in the last 10s.
	// Alternatives are
	// NoResponse (just the TCP ACK) and WaitForLocal (just the local commit needs
	// to succeed)
	config.Producer.RequiredAcks = sarama.WaitForAll

	/***** Create a client. *****/
	// Client is optional: you can create consumers and producers directly.
	// However the best practice (https://github.com/Shopify/sarama/issues/1325)
	// is to use a client to create consumers and producers. I don't yet know why.
	// It does give you the ability to get cluster-level information, e.g., oldest
	// and newest offsets.
	client, err := sarama.NewClient(brokers, config)
	if err != nil {
		log.Fatalf("couldn't create client: %v", err)
	}

	/***** Create a producer. *****/
	// The sync producer is a thin wrapper around the async producer.
	// We could alternatively create it without the client, and provide the list
	// of brokers and the config here.
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		log.Fatalf("couldn't create producer: %v", err)
	}
	defer producer.Close()

	/***** Write a message to a topic *****/
	message1 := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder("burritos are nice " + time.Now().String()),
	}
	message2 := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder("tacos are good too " + time.Now().String()),
	}
	for _, message := range []*sarama.ProducerMessage{message1, message2} {
		partition, offset, err := producer.SendMessage(message)
		if err != nil {
			log.Fatalf("couldn't write a message: %v", err)
		}
		fmt.Printf("Wrote to partition %d at offset %d\n", partition, offset)
	}

	/***** Print stats about messages in the queue. *****/
	var i int32
	for i = 0; i < 2; i++ {
		oldestOffset, err := client.GetOffset(topic, i, sarama.OffsetOldest)
		if err != nil {
			log.Fatalf("couldn't get oldest offset: %v", err)
		}

		newestOffset, err := client.GetOffset(topic, i, sarama.OffsetNewest)
		if err != nil {
			log.Fatalf("couldn't get newest offset: %v", err)
		}
		fmt.Printf("Partition: %d, Oldest: %d, Newest: %d\n", i, oldestOffset, newestOffset)
	}

	/***** Create a consumer. *****/
	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		log.Fatalf("couldn't create consumer: %v", err)
	}
	defer consumer.Close()

	/***** Find all partitions for the topic. *****/
	partitions, err := consumer.Partitions(topic)
	if err != nil {
		log.Fatalf("Error retrieving partitionList: %v", err)
	}
	fmt.Printf("Found all partitions: %+v\n", partitions)

	/***** Read messages from all of the partitions. *****/
	consumerMessages := make(chan *sarama.ConsumerMessage, 256)
	consumerErrors := make(chan *sarama.ConsumerError, 256)

	pc1, err := consumer.ConsumePartition(topic, int32(0), sarama.OffsetOldest)
	if err != nil {
		log.Fatalf("Error consuming partition: %v", err)
	}
	pc2, err := consumer.ConsumePartition(topic, int32(1), sarama.OffsetOldest)
	if err != nil {
		log.Fatalf("Error consuming partition: %v", err)
	}

	for _, partitionConsumer := range []sarama.PartitionConsumer{pc1, pc2} {
		go func(pc sarama.PartitionConsumer) {
			for consumerMessage := range pc.Messages() {
				consumerMessages <- consumerMessage
				fmt.Printf("Received from partition %d => %s\n", consumerMessage.Partition, string(consumerMessage.Value))
			}
		}(partitionConsumer)

		go func(pc sarama.PartitionConsumer) {
			for consumerError := range pc.Errors() {
				consumerErrors <- consumerError
				fmt.Printf("Error: %v\n", consumerError.Err)
			}
		}(partitionConsumer)
	}

	/***** Sleep for a minute to wait for messages to come in. *****/
	time.Sleep(60 * time.Second)

}
