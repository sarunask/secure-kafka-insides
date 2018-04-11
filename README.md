# secure-kafka-insides
Get information from secured TLS enforced Kafka with ACL's enforced.

To be run as long running REST application in Docker.

## Configuration

Configuration is done via Docker environment variables:
1 VAULT_ADDR - address of Hashicorp Vault
1 VAULT_PKI_ENDPOINT - where to write to get Client Certificate
1 VAULT_TOKEN - token to access Vault