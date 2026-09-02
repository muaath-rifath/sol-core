package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type MessageHandler func(topic string, payload []byte)

type Client struct {
	client         pahomqtt.Client
	messageHandler MessageHandler
}

func NewClient(brokerURL, clientID, caCertPath, clientCertPEM, clientKeyPEM string) (*Client, error) {
	broker, err := url.Parse(brokerURL)
	if err != nil || broker.Scheme == "" {
		return nil, fmt.Errorf("parse mqtt broker URL: %q", brokerURL)
	}

	useTLS := broker.Scheme == "ssl" || broker.Scheme == "tls"
	if !useTLS && broker.Scheme != "tcp" {
		return nil, fmt.Errorf("unsupported mqtt broker scheme %q", broker.Scheme)
	}

	var tlsConfig *tls.Config
	if useTLS {
		if caCertPath == "" || clientCertPEM == "" || clientKeyPEM == "" {
			return nil, fmt.Errorf("mTLS is required for %s broker URLs", broker.Scheme)
		}
		tlsConfig, err = createTLSConfig(caCertPath, clientCertPEM, clientKeyPEM, broker.Hostname())
		if err != nil {
			return nil, fmt.Errorf("create tls config: %w", err)
		}
	}

	opts := pahomqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetCleanSession(false)
	if tlsConfig != nil {
		opts.SetTLSConfig(tlsConfig)
	}

	c := &Client{}

	opts.SetDefaultPublishHandler(func(_ pahomqtt.Client, msg pahomqtt.Message) {
		if c.messageHandler != nil {
			c.messageHandler(msg.Topic(), msg.Payload())
		}
	})

	opts.SetOnConnectHandler(func(_ pahomqtt.Client) {
		slog.Info("mqtt connected", "broker", brokerURL)
	})

	opts.SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
		slog.Warn("mqtt connection lost", "error", err)
	})

	client := pahomqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect: %w", token.Error())
	}

	c.client = client
	return c, nil
}

func (c *Client) SetMessageHandler(handler MessageHandler) {
	c.messageHandler = handler
}

func (c *Client) Subscribe(topic string, qos byte) error {
	token := c.client.Subscribe(topic, qos, nil)
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt subscribe %s: %w", topic, err)
	}
	slog.Info("mqtt subscribed", "topic", topic)
	return nil
}

func (c *Client) Publish(topic string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	token := c.client.Publish(topic, 1, false, data)
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt publish %s: %w", topic, err)
	}
	return nil
}

func (c *Client) Disconnect() {
	c.client.Disconnect(1000)
}

func createTLSConfig(caCertPath, clientCertPEM, clientKeyPEM, serverName string) (*tls.Config, error) {
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("read ca cert: %w", err)
	}

	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		slog.Warn("failed to load system cert pool, falling back to empty pool", "error", err)
		caCertPool = x509.NewCertPool()
	}
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.X509KeyPair([]byte(clientCertPEM), []byte(clientKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse backend client key pair: %w", err)
	}

	return &tls.Config{
		RootCAs:            caCertPool,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		ServerName:         serverName,
	}, nil
}
