package main

import (
	"log"
	"time"

	"github.com/Shopify/sarama"
)

func main() {
	// Kafkaブローカーのアドレスとポートを設定します
	brokers := []string{"kafka-cluster-0.kafka-cluster-headless.kafka.svc.cluster.local:9092"}

	// Kafkaプロデューサーの設定を作成します
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	// Kafkaブローカーに接続します
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// 5秒ごとにメッセージを送信します
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 送信するメッセージを作成します
		topic := "topic-otel"
		message := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.StringEncoder("Hello, Kafka x OTel !!"),
		}

		// メッセージを送信します
		partition, offset, err := producer.SendMessage(message)
		if err != nil {
			log.Fatal(err)
		}

		// 送信結果をログに出力します
		log.Printf("Message sent topic: %s successfully! Partition: %d, Offset: %d", topic, partition, offset)
	}
}
