package main

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/sarunask/secure-kafka-insides/kafka"
	"log"
	"os"
	"strings"
	"github.com/sarunask/secure-kafka-insides/security"
	"time"
	"context"
)

func main() {
	if os.Getenv("VAULT_ADDR") == "" ||
		os.Getenv("VAULT_TOKEN") == "" ||
		os.Getenv("VAULT_TOKEN_RENEW_PERIOD") == "" ||
		os.Getenv("VAULT_PKI_ISSUE_ENDPOINT") == "" ||
		os.Getenv("KAFKA_PKI_BASE_FQDN") == "" ||
		os.Getenv("KAFKA_PKI_MANAGER_NAME") == "" ||
		os.Getenv("KAFKA_BROKERS") == "" {
		log.Fatal("Require such env variables: VAULT_TOKEN, VAULT_ADDR, " +
			"VAULT_PKI_ISSUE_ENDPOINT, KAFKA_PKI_BASE_FQDN, KAFKA_PKI_MANAGER_NAME, " +
			"KAFKA_BROKERS, VAULT_TOKEN_RENEW_PERIOD")
	}
	if os.Getenv("VERBOSE") == "1" {
		sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	}
	//Get Context shared between routines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished

	go security.RenewTokenIfNeeded(ctx)

	//Get new TLS config
	securityConfig := security.NewConfig()

	time.Sleep(40*time.Second)

	brokers := os.Getenv("KAFKA_BROKERS")

	brokerList := strings.Split(brokers, ",")
	log.Printf("Kafka brokers: %s", strings.Join(brokerList, ", "))

	configClient := kafka.Config{
		BrokerList: brokerList,
	}

	adminClient := configClient.NewClient(securityConfig)
	defer func() {
		if err := adminClient.Close(); err != nil {
			log.Panic(err)
		}
	}()
	topics, err := adminClient.Topics()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(topics)

	configBroker := kafka.Config{
		BrokerList: configClient.BrokerList,
		TLS:  configClient.TLS,
	}

	describeAclsReq := &sarama.DescribeAclsRequest{
		AclFilter: sarama.AclFilter{
			ResourceType:   sarama.AclResourceTopic,
			PermissionType: sarama.AclPermissionAny,
			Operation:      sarama.AclOperationAny,
		},
	}

	broker := configBroker.NewBroker(securityConfig)
	if err != nil {
		log.Fatalln("Failed to open Sarama broker:", err)
	}
	defer func() {
		if err := broker.Close(); err != nil {
			log.Panic(err)
		}
	}()

	describeAclsResp, err := broker.DescribeAcls(describeAclsReq)
	if err != nil {
		log.Fatal(err)
	}
	acls := describeAclsResp.ResourceAcls
	log.Print(acls)
}
