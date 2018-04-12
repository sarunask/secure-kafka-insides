package web

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/sarunask/secure-kafka-insides/kafka"
	"github.com/sarunask/secure-kafka-insides/security"
	"net/http"
	"sort"
)

var KafkaClient sarama.Client

func RootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
}

func Topics(w http.ResponseWriter, _ *http.Request) {
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
