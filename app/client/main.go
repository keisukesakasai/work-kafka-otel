package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/Shopify/sarama"
)

func main() {
	// kafka 設定
	brokers := []string{"kafka-cluster-0.kafka-cluster-headless.kafka.svc.cluster.local:9092"}

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// Echo サーバー起動
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// リクエストのボディを取得します
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// 送信するメッセージを作成します
		topic := "topic-otel"
		msg := fmt.Sprintf("Hello, Kafka x OTel ! message: %s", string(body))
		message := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.StringEncoder(msg),
		}

		// メッセージを送信します
		partition, offset, err := producer.SendMessage(message)
		if err != nil {
			log.Fatal(err)
		}

		// 送信結果をログに出力します
		log.Printf("Message sent topic: %s successfully! Partition: %d, Offset: %d", topic, partition, offset)

		// レスポンスとして成功を返します
		w.WriteHeader(http.StatusOK)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
