package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Shopify/sarama"
)

func main() {
	// Kafkaブローカーのアドレスとポートを設定します
	brokers := []string{"kafka-cluster-0.kafka-cluster-headless.kafka.svc.cluster.local:9092"}

	// Kafkaコンシューマーの設定を作成します
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	// Kafkaブローカーに接続します
	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// 受信するトピックとパーティションを指定します
	topic := "topic-otel"
	partition := int32(0)

	// オフセットを指定してパーティションのコンシューマーを作成します
	partitionConsumer, err := consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
	if err != nil {
		log.Fatal(err)
	}

	// メッセージ受信のゴルーチンを開始します
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range partitionConsumer.Messages() {
			log.Printf("Received message: Topic = %s, Partition = %d, Offset = %d, Key = %s, Value = %s",
				msg.Topic, msg.Partition, msg.Offset, string(msg.Key), string(msg.Value))
		}
	}()

	// OSのシグナルを待ちます
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

	// ゴルーチンの終了を待ちます
	wg.Wait()
}
