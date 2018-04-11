package security

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/hashicorp/vault/api"
	"io/ioutil"
	"log"
	"os"
	"time"
	"context"
)

type Config struct {
	TLS *tls.Config
	Cert string
	CACert string
	PrivateKey string
}

func NewConfig() *Config  {
	c := &Config{}

	return c
}

//Function would get Vault Logical client
func getVaultLogical() (*api.Logical, error) {
	client, err := api.NewClient(nil)
	if err != nil {
		return nil, fmt.Errorf("getVaultLogical: couldn't get Vault client")
	}
	return client.Logical(), nil
}

//Function would check if certificate has expired
func hasCertificateExpired(rootCAFile, certFile *string) bool {
	rootPEM, err := ioutil.ReadFile(*rootCAFile)
	if err != nil {
		log.Print("failed to read rootCA from file: ", *rootCAFile)
		return true
	}
	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM(rootPEM); !ok {
		log.Fatal("failed to parse root certificate")
	}
	certPEM, err := ioutil.ReadFile(*certFile)
	if err != nil {
		log.Print("failed to read certificate from file: ", rootCAFile)
		return true
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		log.Fatal("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Fatal("failed to parse certificate: " + err.Error())
	}
	opts := x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: time.Now(),
	}
	if _, err := cert.Verify(opts); err != nil {
		log.Print("failed to verify certificate: " + err.Error())
		return true
	}
	return false
}

//Go routing, which would renew Vault token on periodic basis
//Routine would be canceled by message in done channel
func RenewTokenIfNeeded(ctx context.Context) {
	client, err := api.NewClient(nil)
	if err != nil {
		log.Fatal("getVaultLogical: couldn't get Vault client")
	}
	//Get middleman structure to TokenAuth
	auth := client.Auth()
	//VAULT_TOKEN_RENEW_PERIOD must be valid duration,
	// with s,m,h as defined in https://golang.org/pkg/time/#ParseDuration
	renewPeriod, err := time.ParseDuration(os.Getenv("VAULT_TOKEN_RENEW_PERIOD"))
	if err != nil {
		log.Fatal("can't parse VAULT_TOKEN_RENEW_PERIOD")
	}
	ticker := time.NewTicker(renewPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Print("ending token renewal")
			return
		case <-ticker.C:
			log.Print("starting token renew")
			tokenAuth := auth.Token()
			_, err := tokenAuth.RenewSelf(1)
			if err != nil {
				log.Print("error renew token: ", err)
				return
			}
		}
	}
}

//Function would go to Vault to fetch new certificate
func getNewCertificateFromVault(caFile, certFile, keyFile *string) {
	vaultLogical, err := getVaultLogical()
	if err != nil {
		log.Print("couldn't get Vault client: ", err)
		return
	}
	secret, err := vaultLogical.Write(os.Getenv("VAULT_PKI_ISSUE_ENDPOINT"),
		map[string]interface{}{
			"common_name": fmt.Sprintf("%s.%s",
				os.Getenv("KAFKA_PKI_MANAGER_NAME"),
				os.Getenv("KAFKA_PKI_BASE_FQDN")),
			"ttl": "1h",
		})
	if err != nil {
		log.Print("Couldn't issue new TLS certificates from Vault: ", err)
		return
	}

	ioutil.WriteFile(*caFile, []byte(secret.Data["issuing_ca"].(string)), 0600)
	ioutil.WriteFile(*certFile, []byte(secret.Data["certificate"].(string)), 0600)
	ioutil.WriteFile(*keyFile, []byte(secret.Data["private_key"].(string)), 0600)
}

//Function would be called upon Config struct. If it's already pre-initialized, code
// would do nothing. If struct is empty, initialization would be done:
// 1. Permanent certificate store is read
// 2. If certificates are expired Vault is called
// 1. If certificates
// 2. New management TLS certificates with name KAFKA_PKI_MANAGER_NAME.KAFKA_PKI_MANAGER_NAME would be
// issued. It's TTL by default is 120h
// 3. Certificates would be transfered to TLS.Config structure, which would be used to connect to
// Kafka cluster
func (c *Config) NewConfig() {
	//Only create TLS config if it's empty
	if c.TLS != nil {
		return
	}
	basePath := os.Getenv("CLIENT_PERM_STORAGE")
	if len(basePath) == 0 {
		basePath = "."
	}
	caFile := fmt.Sprintf("%s/ca.pem", basePath)
	certFile := fmt.Sprintf("%s/cert.pem", basePath)
	keyFile := fmt.Sprintf("%s/key.pem", basePath)

	if hasCertificateExpired(&caFile, &certFile) {
		getNewCertificateFromVault(&caFile, &certFile, &keyFile)
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatal(err)
	}

	rootPEM, err := ioutil.ReadFile(caFile)
	if err != nil {
		log.Print("failed to read rootCA from file: ", caFile)
		return
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(rootPEM)

	c.TLS = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}
}
