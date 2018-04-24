package security

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/hashicorp/vault/api"
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

var SecConfig *Config

//Function would be called to return Config struct. Initialization would be done:
// 1. Vault is called to get new certs
// 2. New management TLS certificates with name KAFKA_PKI_MANAGER_NAME.KAFKA_PKI_MANAGER_NAME would be
// issued. It's TTL by default is 120h
// 3. Certificates are stored into structure as strings
// 4. Certificates would be parsed to TLS.Config structure, which would be used to connect to
// Kafka cluster
func NewConfig() *Config  {
	//Initialize config
	c := &Config{}

	if err := c.getAndParseCerts(); err != nil {
		log.Fatal(err)
	}

	return c
}

//Function would check if certificate has expired
func (c *Config) HasCertificateExpired() bool {
	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM([]byte(c.CACert)); !ok {
		log.Fatal("failed to parse root certificate")
	}
	block, _ := pem.Decode([]byte(c.Cert))
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
func RenewToken(ctx context.Context) {
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

//Go routing, which would renew TLS Certificate on periodic basis
func RenewCertificate(ctx context.Context, c *Config) {
	renewPeriod, err := time.ParseDuration(os.Getenv("VAULT_TLS_RENEW_PERIOD"))
	if err != nil {
		log.Fatal("Can't parse VAULT_TLS_RENEW_PERIOD")
	}
	//Start renewal 10 seconds before TTL of cert
	ticker := time.NewTicker(renewPeriod-(10*time.Second))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Print("Ending TLS renewal")
			return
		case <-ticker.C:
			if ! c.HasCertificateExpired() {
				continue
			}
			log.Print("starting certificate renew")
			if err := c.getAndParseCerts(); err != nil {
				log.Fatal(err)
			}
		}
	}
}

//Function would go to Vault to fetch new certificate
func (c *Config) getNewCertificateFromVault() error {
	vaultLogical, err := getVaultLogical()
	if err != nil {
		return fmt.Errorf("couldn't get Vault client: %s", err)
	}
	secret, err := vaultLogical.Write(os.Getenv("VAULT_PKI_ISSUE_ENDPOINT"),
		map[string]interface{}{
			"common_name": fmt.Sprintf("%s.%s",
				os.Getenv("KAFKA_PKI_MANAGER_NAME"),
				os.Getenv("KAFKA_PKI_BASE_FQDN")),
			"ttl": os.Getenv("VAULT_TLS_RENEW_PERIOD"),
		})
	if err != nil {
		return fmt.Errorf("couldn't issue new TLS certificates from Vault: %s", err)
	}

	c.CACert = secret.Data["issuing_ca"].(string)
	c.Cert = secret.Data["certificate"].(string)
	c.PrivateKey = secret.Data["private_key"].(string)
	return nil
}

func (c *Config) getAndParseCerts() error {
	var cert tls.Certificate
	var keyDERBlock, certDERBlock *pem.Block
	var err error

	if err := c.getNewCertificateFromVault(); err != nil {
		return err
	}
	//Parse and assign public key
	if certDERBlock, _ = pem.Decode([]byte(c.Cert)); certDERBlock == nil {
		return fmt.Errorf("NewConfig: failed to parse cert PEM data")
	}
	cert.Certificate = append(cert.Certificate, certDERBlock.Bytes)

	//Parse and assign private key
	if keyDERBlock, _ = pem.Decode([]byte(c.PrivateKey)); keyDERBlock == nil {
		return fmt.Errorf("NewConfig: failed to parse private key PEM data")
	}
	if cert.PrivateKey, err = x509.ParsePKCS1PrivateKey(keyDERBlock.Bytes); err != nil {
		return fmt.Errorf("NewConfig: failed to parse private key data")
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(c.CACert))

	c.TLS = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}
	return nil
}

//Function would get Vault Logical client
func getVaultLogical() (*api.Logical, error) {
	client, err := api.NewClient(nil)
	if err != nil {
		return nil, fmt.Errorf("getVaultLogical: couldn't get Vault client")
	}
	return client.Logical(), nil
}

