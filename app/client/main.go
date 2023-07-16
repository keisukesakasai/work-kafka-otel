package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	_ "time/tzdata"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"
	otelsarama "go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/otelsarama"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var tracer = otel.Tracer("kafka-producer")

func initProvider(ctx context.Context) (func(context.Context) error, error) {
	var tracerProvider *sdktrace.TracerProvider

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("kakfa-producer"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	conn, err := grpc.DialContext(ctx, "otelcol-collector.observability.svc.cluster.local:4317", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		fmt.Println("failed to create gRPC connection to collector: %w", err)
	}

	// Set up a trace exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tracerProvider.Shutdown, nil
}

func main() {
	// timezone 設定
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatal("%w", err)
	}
	log.Printf("%v", loc)

	// otel 設定
	ctx := context.Background()
	shutdown, err := initProvider(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("failed to shutdown TracerProvider: %w", err)
		}
	}()

	// kafka 設定
	brokerList := []string{"kafka-cluster-0.kafka-cluster-headless.kafka.svc.cluster.local:9092"}
	log.Printf("Kafka brokers: %s", strings.Join(brokerList, ", "))

	// Http server
	r := gin.New()
	r.Use(otelgin.Middleware("kafka producer"))
	r.GET("/", func(c *gin.Context) {
		// リクエストのボディを取得します
		_, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			http.Error(c.Writer, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// Create child span
		ctx, span := tracer.Start(c.Request.Context(), "produce message")
		defer span.End()

		// kafka producer
		producer, err := newAccessLogProducer(brokerList)
		if err != nil {
			log.Fatal(err)
		}

		// 送信するメッセージを作成します
		rng := rand.New(rand.NewSource(time.Now().Unix()))
		topic := "topic-otel"
		msg := sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder("random_number"),
			Value: sarama.StringEncoder(fmt.Sprintf("%d", rng.Intn(1000))),
		}
		otel.GetTextMapPropagator().Inject(ctx, otelsarama.NewProducerMessageCarrier(&msg))

		// メッセージを送信します
		producer.Input() <- &msg
		successMsg := <-producer.Successes()
		log.Printf("Message sent topic: %s successfully! Partition: %d, Offset: %d", topic, successMsg.Partition, successMsg.Offset)

		err = producer.Close()
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			log.Fatalln("Failed to close producer:", err)
		}
	})

	r.Run(":8080")
}

func newAccessLogProducer(brokerList []string) (sarama.AsyncProducer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_5_0_0
	// So we can know the partition and offset of messages.
	config.Producer.Return.Successes = true

	producer, err := sarama.NewAsyncProducer(brokerList, config)
	if err != nil {
		return nil, fmt.Errorf("starting Sarama producer: %w", err)
	}

	// Wrap instrumentation
	producer = otelsarama.WrapAsyncProducer(config, producer)

	// We will log to STDOUT if we're not able to produce messages.
	go func() {
		for err := range producer.Errors() {
			log.Println("Failed to write message:", err)
		}
	}()

	return producer, nil
}
