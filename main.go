package main

import (
	"github.com/Shopify/sarama"
	"crypto/x509"
	"io/ioutil"
	"crypto/tls"
	"log"
	"flag"
	"os"
	"strings"
	"fmt"
)

var (
	caFile 		= flag.String("ca", "ca.pem", "The optional certificate authority file for TLS client authentication")
	certFile  	= flag.String("cert", "cert.pem", "The optional certificate file for client authentication")
	keyFile 	= flag.String("key", "priv.pem", "The optional key file for client authentication")
	brokers		= flag.String("brokers","127.0.0.1:9094", "The Kafka brokers to connect to, as a comma separated list")
	verbose   	= flag.Bool("verbose", true, "Turn on Sarama logging")
)

func createTlsConfiguration() (t *tls.Config) {
	if *certFile != "" && *keyFile != "" && *caFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatal(err)
		}

		caCert, err := ioutil.ReadFile(*caFile)
		if err != nil {
			log.Fatal(err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		t = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: true,
		}
	}
	// will be nil by default if nothing is provided
	return t
}

func newAdminClient(brokerList []string) sarama.Client {

	// For the data collector, we are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Return.Successes = true
	config.ClientID = "acl_reader"
	tlsConfig := createTlsConfiguration()
	if tlsConfig != nil {
		config.Net.TLS.Config = tlsConfig
		config.Net.TLS.Enable = true
	}

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.
	adminClient, err := sarama.NewClient(brokerList, config)
	if err != nil {
		log.Fatalln("Failed to start Sarama admin client:", err)
	}

	return adminClient
}

func newBroker(brokerList []string) *sarama.Broker {

	// For the data collector, we are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Return.Successes = true
	config.Version = sarama.V1_0_0_0
	config.ClientID = "acl_reader"
	config.Net.TLS.Config = createTlsConfiguration()
	config.Net.TLS.Enable = true

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.
	broker := sarama.NewBroker(brokerList[0])
	err := broker.Open(config)
	if err != nil {
		log.Fatalln("Failed to open Sarama broker:", err)
	}

	return broker
}

func main()  {
	flag.Parse()

	if *verbose {
		sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	}

	if *brokers == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	brokerList := strings.Split(*brokers, ",")
	log.Printf("Kafka brokers: %s", strings.Join(brokerList, ", "))

	adminClient := newAdminClient(brokerList)
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
	brokers := adminClient.Brokers()
	for i := 0; i < len(brokers); i++ {
		connected, err := brokers[i].Connected()
		if err != nil {
			continue
		}
		log.Printf("Broker %v is connected? %v", i, connected)
	}


	describeAclsReq := &sarama.DescribeAclsRequest{
			AclFilter: sarama.AclFilter{
			ResourceType: sarama.AclResourceTopic,
			PermissionType: sarama.AclPermissionAny,
			Operation: sarama.AclOperationAny,
		},
	}

	broker :=  newBroker(brokerList)
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
