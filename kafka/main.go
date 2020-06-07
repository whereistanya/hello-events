package main

import (
	"fmt"
	"log"
	_ "os"

	"github.com/Shopify/sarama"
)

func main() {

	brokers := []string{"localhost:9092"}

	/**** Create a config. *****/
	config := sarama.NewConfig()
	config.ClientID = "the_kafkarator"
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
		log.Fatal("couldn't create client: %v", err)
	}

	/***** Create a producer. *****/
	// The sync producer is a thin wrapper around the async producer.
	// We could alternatively create it without the client, and provide the list
	// of brokers and the config here.
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		log.Fatal("couldn't create producer: %v", err)
	}
	defer producer.Close()

	/***** Write a message to a topic *****/

	topic := "burritos"

	// Just out of interest, some stats.
	fmt.Println("Before we write anything, the offsets are...")
	var i int32
	for i = 0; i < 2; i++ {
		// TODO: ignoring errors is reprehensible.
		oldestOffset, _ := client.GetOffset(topic, i, sarama.OffsetOldest)
		newestOffset, _ := client.GetOffset(topic, i, sarama.OffsetNewest)
		fmt.Printf("Partition: %d, Oldest: %d, Newest: %d\n", i, oldestOffset, newestOffset)
	}
	message1 := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder("burritos are nice"),
	}
	message2 := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder("tacos are good too"),
	}
	for _, message := range []*sarama.ProducerMessage{message1, message2} {
		partition, offset, err := producer.SendMessage(message)
		if err != nil {
			log.Fatal("couldn't write a message: %v", err)
		}
		fmt.Printf("Wrote to partition %d at offset %d\n", partition, offset)
	}

	for i = 0; i < 2; i++ {
		// TODO: ignoring errors is reprehensible.
		oldestOffset, _ := client.GetOffset(topic, i, sarama.OffsetOldest)
		newestOffset, _ := client.GetOffset(topic, i, sarama.OffsetNewest)
		fmt.Printf("Partition: %d, Oldest: %d, Newest: %d\n", i, oldestOffset, newestOffset)
	}

	/***** Create a consumer. *****/
	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		log.Fatal("couldn't create consumer: %v", err)
	}
	defer consumer.Close()

	/***** Read the message *****/

	/*
		partitions, err := consumer.Partitions(topic) //get all partitions on the given topic
		if err != nil {
			fmt.Println("Error retrieving partitionList ", err)
		}
		fmt.Printf("Found all partitions: %+v\n", partitions)

		for _, partition := range partitions {
			fmt.Println("Partition is:", partition)
			pc, err := consumer.ConsumePartition(topic, partition, oldestOffset)
			fmt.Println("Comsumed partition is:", pc)
			if err != nil {
				fmt.Println("Error consuming partition ", err)
			}

			go func(pc sarama.PartitionConsumer) {
				for message := range pc.Messages() {
					fmt.Printf("Received: %s\n", string(message.Value))
				}
			}(pc)
		}
	*/

	fmt.Println("End!")

}
