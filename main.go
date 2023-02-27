package main

import (
	"encoding/json"
	"flag"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"log"
	"mqtt_web_terminal/pkg/client"
	"mqtt_web_terminal/pkg/tty"
	"os"
	"os/signal"
	"syscall"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	mqtt.ERROR = log.New(os.Stdout, "[ERROR] ", 0)
	//mqtt.CRITICAL = log.New(os.Stdout, "[CRIT] ", 0)
	//mqtt.WARN = log.New(os.Stdout, "[WARN]  ", 0)
	//mqtt.DEBUG = log.New(os.Stdout, "[DEBUG] ", 0)

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

func main() {

	var (
		broker   string
		username string
		password string
		name     string
		command  string
	)

	flag.StringVar(&broker, "broker", "", "wss[mqtt|mqtts]://xx:port")
	flag.StringVar(&username, "username", "", "用户名")
	flag.StringVar(&password, "password", "", "密码")
	flag.StringVar(&name, "name", "", "设备名, topic 拼接为: /shell/{name}/input[output]")
	flag.StringVar(&command, "command", "", "终端shell")
	flag.Parse()

	if name == "" || broker == "" || username == "" || password == "" {
		log.Println("parameter error")
		os.Exit(1)
	}
	inputTopic := fmt.Sprintf("/shell/%s/input", name)
	outputTopic := fmt.Sprintf("/shell/%s/output", name)

	term, err := tty.New(command)
	if err != nil {
		panic(err)
	}

	cli := client.New(username, password, name, broker)

	cli.Sub(inputTopic, 1, func(message mqtt.Message) {
		msg := tty.Message{}
		err := json.Unmarshal(message.Payload(), &msg)
		if err != nil {
			log.Printf("message err: %v", err)
			return
		}
		term.Input() <- msg
	})

	go func() {
		for msg := range term.Output() {
			msgData, _ := json.Marshal(msg)
			cli.Publish(outputTopic, 0, string(msgData))
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	<-c
	log.Printf("stopping...")
	term.Close()
}
