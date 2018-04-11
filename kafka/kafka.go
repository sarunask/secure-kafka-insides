package kafka

import (
	"github.com/Shopify/sarama"
	"github.com/sarunask/secure-kafka-insides/security"
	"log"
	"crypto/tls"
)

type Config struct {
	TLS  *tls.Config
	BrokerList []string
}

func (c *Config) NewClient(security *security.Config) sarama.Client {

	// For the data collector, we are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Return.Successes = true
	config.ClientID = "acl_reader"
	c.TLS = security.TLS
	config.Net.TLS.Config = c.TLS
	config.Net.TLS.Enable = true

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.
	adminClient, err := sarama.NewClient(c.BrokerList, config)
	if err != nil {
		log.Fatalln("Failed to start Sarama admin client: ", err)
	}

	return adminClient
}

func (c *Config) NewBroker(security *security.Config) *sarama.Broker {

	// For the data collector, we are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Return.Successes = true
	config.Version = sarama.V1_0_0_0
	config.ClientID = "acl_reader"
	c.TLS = security.TLS
	config.Net.TLS.Config = c.TLS
	config.Net.TLS.Enable = true

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.
	broker := sarama.NewBroker(c.BrokerList[0])
	err := broker.Open(config)
	if err != nil {
		log.Fatalln("Failed to open Sarama broker:", err)
	}

	return broker
}
