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
	"html/template"
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
	templates = template.Must(template.ParseFiles(
		"templates/index.html",
		"templates/client_rights.html",
		"templates/topic_rights.html",
		"templates/topics.html",
		"templates/acls.html",
		"templates/topic_acls.html",
		))
)

type Data struct {
	Topics    *[]string
	ACLs      *[]ACL
	UserCN    *string
	TopicName *string
}

type ACL struct {
	Principal  string
	TopicName  string
	Permission string
	Operation  string
	Host       string
}

func renderTemplate(w http.ResponseWriter, tmpl string, someData *Data) {
	err := templates.ExecuteTemplate(w, tmpl+".html", someData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RootHandler(w http.ResponseWriter, _ *http.Request) {
	renderTemplate(w, "index", nil)
}

func EnterClientCNHandler(w http.ResponseWriter, _ *http.Request) {
	renderTemplate(w, "client_rights", nil)
}

func EnterTopicHandler(w http.ResponseWriter, _ *http.Request) {
	renderTemplate(w, "topic_rights", nil)
}

func KafkaTopics(w http.ResponseWriter, _ *http.Request) {
	i := 0
LOOP:
	i = i + 1
	topics, err := KafkaClient.Topics()
	if err != nil {
		if i == 1 {
			//First time try to re-initiate, maybe TLS have changed
			KafkaClient.Close()
			KafkaClient = kafka.ConfigClient.NewClient(security.SecConfig)
			goto LOOP
		} else {
			//Second failure - report
			fmt.Fprintf(w, "%v", err)
		}
	}
	sort.Strings(topics)
	//Remove first as it's __consumer_offsets - Kafka internal one
	if len(topics) != 0 && strings.Contains(topics[0], "__consumer_offsets") {
		topics = topics[1:]
	}
	data := Data{}
	data.Topics = &topics
	renderTemplate(w, "topics", &data)
}

func KafkaUsersAcls(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	user := strings.TrimSpace(r.Form.Get("userCN"))
	userCN := fmt.Sprintf("CN=%s", user)

	describeAclsResp, err := getKafkaAclsForTopics()
	if err != nil {
		log.Fatal(err)
	}
	acls := describeAclsResp.ResourceAcls

	principal := fmt.Sprintf("User:%s", userCN)
	data := Data{
		UserCN: &userCN,
	}
	dataAcls := make([]ACL, 0)
	for _, acl := range acls {
		for _, innerAcl := range acl.Acls {
			if strings.Compare(innerAcl.Principal, principal) != 0 {
				continue
			}
			acl := ACL{
				TopicName: acl.ResourceName,
				Permission: aclPermissions[innerAcl.PermissionType],
				Operation: aclOperations[innerAcl.Operation],
				Host: innerAcl.Host,
			}
			dataAcls = append(dataAcls, acl)
		}
	}
	data.ACLs = &dataAcls
	renderTemplate(w, "acls", &data)
}

func KafkaTopicsAcls(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	topic := strings.TrimSpace(r.Form.Get("topic"))

	describeAclsResp, err := getKafkaAclsForTopics()
	if err != nil {
		log.Fatal(err)
	}
	acls := describeAclsResp.ResourceAcls

	data := Data{
		TopicName: &topic,
	}
	dataAcls := make([]ACL, 0)
	for _, acl := range acls {
		if strings.Compare(acl.ResourceName, topic) != 0 {
			continue
		}
		for _, innerAcl := range acl.Acls {
			acl := ACL{
				Principal: innerAcl.Principal,
				Permission: aclPermissions[innerAcl.PermissionType],
				Operation: aclOperations[innerAcl.Operation],
				Host: innerAcl.Host,
			}
			dataAcls = append(dataAcls, acl)
		}
	}
	data.ACLs = &dataAcls
	renderTemplate(w, "topic_acls", &data)
}

func getKafkaAclsForTopics() (*sarama.DescribeAclsResponse, error)  {
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

	return broker.DescribeAcls(describeAclsReq)
}
