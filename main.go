package main

import (
	"github.com/Shopify/sarama"
	"github.com/sarunask/secure-kafka-insides/kafka"
	"log"
	"os"
	"strings"
	"github.com/sarunask/secure-kafka-insides/security"
	"context"
	"net/http"
	"github.com/sarunask/secure-kafka-insides/web"
)

func main() {
	if os.Getenv("VAULT_ADDR") == "" ||
		os.Getenv("VAULT_TOKEN") == "" ||
		os.Getenv("VAULT_TOKEN_RENEW_PERIOD") == "" ||
		os.Getenv("VAULT_TLS_RENEW_PERIOD") == "" ||
		os.Getenv("VAULT_PKI_ISSUE_ENDPOINT") == "" ||
		os.Getenv("KAFKA_PKI_BASE_FQDN") == "" ||
		os.Getenv("KAFKA_PKI_MANAGER_NAME") == "" ||
		os.Getenv("KAFKA_BROKERS") == "" {
		log.Fatal("Require such env variables: VAULT_ADDR, VAULT_TOKEN, " +
			"VAULT_PKI_ISSUE_ENDPOINT, KAFKA_PKI_BASE_FQDN, KAFKA_PKI_MANAGER_NAME, " +
			"KAFKA_BROKERS, VAULT_TOKEN_RENEW_PERIOD, VAULT_TLS_RENEW_PERIOD")
	}
	if os.Getenv("VERBOSE") == "1" {
		sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	}
	//Get Context shared between routines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished

	go security.RenewToken(ctx)

	//Get new TLS config
	security.SecConfig = security.NewConfig()

	go security.RenewCertificate(ctx, security.SecConfig)

	brokers := os.Getenv("KAFKA_BROKERS")

	brokerList := strings.Split(brokers, ",")
	log.Printf("Kafka brokers: %s", strings.Join(brokerList, ", "))

	kafka.ConfigClient = kafka.NewConfig(brokerList)

	web.KafkaClient = kafka.ConfigClient.NewClient(security.SecConfig)
	defer func() {
		if err := web.KafkaClient.Close(); err != nil {
			log.Panic(err)
		}
	}()

	//Web handlers and server
	http.HandleFunc("/", web.RootHandler)
	http.HandleFunc("/enterUsersCN", web.EnterClientCNHandler)
	http.HandleFunc("/enterTopic", web.EnterTopicHandler)
	http.HandleFunc("/topics", web.KafkaTopics)
	http.HandleFunc("/usersAcls/", web.KafkaUsersAcls)
	http.HandleFunc("/topicsAcls/", web.KafkaTopicsAcls)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
