package client

import (
	"crypto/tls"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"log"
	"time"
)

type Subscribe struct {
	topic          string
	qos            byte
	messageHandler func(message mqtt.Message)
}

type Mqtt struct {
	client     mqtt.Client
	subscribes []*Subscribe
}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	log.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	log.Printf("Connect lost: %v", err)
}

func (m *Mqtt) Sub(topic string, qos byte, f func(message mqtt.Message)) {
	//token := m.client.Subscribe(topic, qos, func(client mqtt.Client, message mqtt.Message) {
	//	f(message)
	//})
	//
	//token.Wait()
	//log.Printf("Subscribed to topic: %s", topic)
	m.subscribes = append(m.subscribes, &Subscribe{
		topic:          topic,
		qos:            qos,
		messageHandler: f,
	})
}

func (m *Mqtt) Publish(topic string, qos byte, message string) {
	token := m.client.Publish(topic, qos, false, message)
	token.Wait()
}

func New(username, password, clientId, broker string) *Mqtt {
	m := &Mqtt{}
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientId)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetKeepAlive(60 * time.Second)

	opts.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})
	opts.SetDefaultPublishHandler(messagePubHandler)

	opts.OnConnect = func(c mqtt.Client) {
		log.Printf("Connected %s", broker)
		for _, subscribe := range m.subscribes {
			log.Printf("Subscribe: %s", subscribe.topic)
			go c.Subscribe(subscribe.topic, subscribe.qos, func(client mqtt.Client, message mqtt.Message) {
				subscribe.messageHandler(message)
			})
		}
	}

	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	
	m.client = client
	return m
}
