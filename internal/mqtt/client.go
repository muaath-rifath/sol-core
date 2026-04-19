package mqtt

import (
	"encoding/json"
	"fmt"
	"log/slog"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type MessageHandler func(topic string, payload []byte)

type Client struct {
	client         pahomqtt.Client
	messageHandler MessageHandler
}

func NewClient(brokerURL, clientID, username, password string) (*Client, error) {
	opts := pahomqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetCleanSession(false)

	if username != "" {
		opts.SetUsername(username).SetPassword(password)
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
