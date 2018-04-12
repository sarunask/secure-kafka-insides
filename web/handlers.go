package web

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/sarunask/secure-kafka-insides/kafka"
	"github.com/sarunask/secure-kafka-insides/security"
	"net/http"
	"sort"
	"log"
	"strings"
)

var (
	KafkaClient sarama.Client
	aclPermissions = map[sarama.AclPermissionType]string{
		sarama.AclPermissionUnknown: "Unknown",
		sarama.AclPermissionAny: "Any",
		sarama.AclPermissionDeny: "Deny",
		sarama.AclPermissionAllow: "Allow",
	}
	aclOperations = map[sarama.AclOperation]string{
		sarama.AclOperationUnknown: "Unknown",
		sarama.AclOperationAny: "Any",
		sarama.AclOperationAll: "All",
		sarama.AclOperationRead: "Read",
		sarama.AclOperationWrite: "Write",
		sarama.AclOperationCreate: "Create",
		sarama.AclOperationDelete: "Delete",
		sarama.AclOperationAlter: "Alter",
		sarama.AclOperationDescribe: "Describe",
		sarama.AclOperationClusterAction: "ClusterAction",
		sarama.AclOperationDescribeConfigs: "Describe Configs",
		sarama.AclOperationAlterConfigs: "Alter Configs",
		sarama.AclOperationIdempotentWrite: "Indepotent Write",
	}
)

func RootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
}

func KafkaTopics(w http.ResponseWriter, _ *http.Request) {
	i := 0
LOOP:
	i = i + 1
	topics, err := KafkaClient.Topics()
	if err != nil {
		if i == 1 {
			//First time try to re-initiate, maybe TLS have changed
			KafkaClient = kafka.ConfigClient.NewClient(security.SecConfig)
			goto LOOP
		} else {
			//Second failure - report
			fmt.Fprintf(w, "%v", err)
		}
	}
	sort.Strings(topics)
	for _, topic := range topics {
		fmt.Fprintf(w, "<b>%s</b><br/>", topic)
	}
}

func KafkaUsersAcls(w http.ResponseWriter, r *http.Request) {
	userCN := r.URL.Path[len("/usersAcls/"):]

	configBroker := kafka.Config{
		BrokerList: kafka.ConfigClient.BrokerList,
		TLS:  kafka.ConfigClient.TLS,
	}

	describeAclsReq := &sarama.DescribeAclsRequest{
		AclFilter: sarama.AclFilter{
			ResourceType:   sarama.AclResourceTopic,
			PermissionType: sarama.AclPermissionAny,
			Operation:      sarama.AclOperationAny,
		},
	}

	broker := configBroker.NewBroker(security.SecConfig)
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
	fmt.Fprintf(w, "Users principal: %v<br/>", userCN)
	principal := fmt.Sprintf("User:%s", userCN)
	for _, acl := range acls {
		for _, innerAcl := range acl.Acls {
			if strings.Compare(innerAcl.Principal, principal) == 0 {
				fmt.Fprintf(w, "Topic: %v, Permission: %v, Operation: %v, Hosts: %v<br/>",
					acl.ResourceName, aclPermissions[innerAcl.PermissionType],
					aclOperations[innerAcl.Operation], innerAcl.Host)
			}
		}
	}
}
