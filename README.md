# secure-kafka-insides
Get information from secured TLS enforced Kafka with ACL's enforced.

To be run as long running REST application in Docker.

## Configuration

Configuration is done via Docker environment variables:
1. VAULT_ADDR - address of Hashicorp Vault
1. VAULT_TOKEN - Periodic token, which could issue Kafka manager TLS certificate
1. VAULT_TOKEN_RENEW_PERIOD - Renewal period of token, should be less that period,
so that server could renew token before it's expire. Should be ending with s,m,h
as defined in [here](https://golang.org/pkg/time/#ParseDuration)
1. VAULT_TLS_RENEW_PERIOD - Period (in hours) ending with `h` like `24h` after which
TLS certificate should be renewed. TLS cert TTL would be issued from Vault with this
value
1. VAULT_PKI_ISSUE_ENDPOINT - where to get certificate from, should be like pki/issue/some_role
1. KAFKA_PKI_BASE_FQDN - TLS certificate CN FQDN consists of 2 parts which are combined in this way:
KAFKA_PKI_MANAGER_NAME.KAFKA_PKI_BASE_FQDN
Here is last part of FQDN
1. KAFKA_PKI_MANAGER_NAME - TLS certificate CN FQDN consists of 2 parts which are combined in this way:
KAFKA_PKI_MANAGER_NAME.KAFKA_PKI_BASE_FQDN
Here is initial part of FQDN
1. KAFKA_BROKERS - Comma separated list of Kafka brokers with ports, like 10.0.0.1:9093,10.0.0.2:9093
Can be single host with port.