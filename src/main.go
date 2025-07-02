package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"gopkg.in/yaml.v2"
)

type SourceData struct {
	Acc  int     `json:"acc"`
	Alt  int     `json:"alt"`
	Batt int     `json:"batt"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type ConvertedData struct {
	GPSAccuracy int     `json:"gps_accuracy"`
	Altitude    int     `json:"altitude"`
	Battery     int     `json:"battery_level"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

type Config struct {
	SourceBroker        string            `yaml:"source_broker"`
	SourcePort          int               `yaml:"source_port"`
	SourceUser          string            `yaml:"source_user"`
	SourcePass          string            `yaml:"source_pass"`
	TargetBroker        string            `yaml:"target_broker"`
	TargetPort          int               `yaml:"target_port"`
	TargetUser          string            `yaml:"target_user"`
	TargetPass          string            `yaml:"target_pass"`
	UseTLS              bool              `yaml:"use_tls"`
	RunMode             string            `yaml:"run_mode"`
	QoS                 int               `yaml:"qos"`
	Debug               bool              `yaml:"debug"`
	Mappings            map[string]string `yaml:"mappings"`
	ExitOnIdle          bool              `yaml:"exit_on_idle"`
	IdleTimeoutSeconds  int               `yaml:"idle_timeout_seconds"`
}

var config Config
var targetClient MQTT.Client
var lastMessageTime time.Time

func loadConfig(filename string) {
	file, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}
	if err := yaml.Unmarshal(file, &config); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}
}

func getBrokerURL(broker string, port int, useTLS bool) string {
	protocol := "mqtt"
	if useTLS {
		protocol = "mqtts"
	}
	return fmt.Sprintf("%s://%s:%d", protocol, broker, port)
}

func configureMQTTClientOptions(broker, clientID, username, password string, useTLS bool) *MQTT.ClientOptions {
	opts := MQTT.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetOrderMatters(false)

	if useTLS {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS13,
		}
		opts.SetTLSConfig(tlsConfig)
	}

	if username != "" && password != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	return opts
}

func messageHandler(client MQTT.Client, msg MQTT.Message) {
	lastMessageTime = time.Now()

	log.Printf("Received message from source topic: %s, payload: %s", msg.Topic(), string(msg.Payload()))

	var source SourceData
	if err := json.Unmarshal(msg.Payload(), &source); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		return
	}

	if source.Lat == 0 || source.Lon == 0 {
		log.Printf("Invalid data received: missing latitude or longitude")
		return
	}

	converted := ConvertedData{
		GPSAccuracy: source.Acc,
		Altitude:    source.Alt,
		Battery:     source.Batt,
		Latitude:    source.Lat,
		Longitude:   source.Lon,
	}

	subTopic := msg.Topic()
	pubTopic, exists := config.Mappings[subTopic]
	if !exists {
		log.Printf("No mapping found for topic: %s", subTopic)
		return
	}

	if config.Debug {
		raw, _ := json.MarshalIndent(source, "", "  ")
		conv, _ := json.MarshalIndent(converted, "", "  ")
		log.Printf("[DEBUG] Original data from %s:\n%s", subTopic, raw)
		log.Printf("[DEBUG] Converted data to %s:\n%s", pubTopic, conv)
	}

	payload, err := json.Marshal(converted)
	if err != nil {
		log.Printf("Error encoding JSON: %v", err)
		return
	}

	token := targetClient.Publish(pubTopic, byte(config.QoS), false, payload)
	token.Wait()
	if token.Error() != nil {
		log.Printf("Failed to publish message to %s: %v", pubTopic, token.Error())
	} else {
		log.Printf("Successfully published to %s: %s", pubTopic, payload)
	}
}

func main() {
	log.Println("Loading configuration...")
	loadConfig("config/config.yaml")
	log.Println("Configuration loaded successfully.")

	// Source broker setup
	sourceBroker := getBrokerURL(config.SourceBroker, config.SourcePort, config.UseTLS)
	log.Printf("Connecting to Source MQTT broker: %s", sourceBroker)
	sourceOpts := configureMQTTClientOptions(sourceBroker, "mqtt_converter", config.SourceUser, config.SourcePass, config.UseTLS)
	sourceOpts.SetDefaultPublishHandler(messageHandler)
	sourceClient := MQTT.NewClient(sourceOpts)
	token := sourceClient.Connect()
	if token.Wait() && token.Error() != nil {
		log.Fatalf("Source MQTT connection failed: %v", token.Error())
	}
	for !sourceClient.IsConnected() {
		log.Println("Waiting for Source MQTT connection to establish...")
		time.Sleep(500 * time.Millisecond)
	}
	log.Println("Connected to Source MQTT broker")

	// Target broker setup
	targetBroker := getBrokerURL(config.TargetBroker, config.TargetPort, config.UseTLS)
	log.Printf("Connecting to Target MQTT broker: %s", targetBroker)
	targetOpts := configureMQTTClientOptions(targetBroker, "mqtt_publisher", config.TargetUser, config.TargetPass, config.UseTLS)
	targetClient = MQTT.NewClient(targetOpts)
	token = targetClient.Connect()
	if token.Wait() && token.Error() != nil {
		log.Fatalf("Target MQTT connection failed: %v", token.Error())
	}
	for !targetClient.IsConnected() {
		log.Println("Waiting for Target MQTT connection to establish...")
		time.Sleep(500 * time.Millisecond)
	}
	log.Println("Connected to Target MQTT broker")

	// Subscribe to topics with retries
	for subTopic := range config.Mappings {
		log.Printf("Subscribing to topic: %s", subTopic)
		for attempt := 1; attempt <= 5; attempt++ {
			if !sourceClient.IsConnected() {
				log.Printf("Client not connected yet. Waiting to subscribe: %s", subTopic)
				time.Sleep(1 * time.Second)
				continue
			}
			token := sourceClient.Subscribe(subTopic, byte(config.QoS), nil)
			token.Wait()
			if token.Error() != nil {
				log.Printf("Subscription attempt %d failed for topic %s: %v", attempt, subTopic, token.Error())
				time.Sleep(1 * time.Second)
			} else {
				log.Printf("Successfully subscribed to topic: %s", subTopic)
				break
			}
		}
	}

	lastMessageTime = time.Now()

	if config.ExitOnIdle && config.IdleTimeoutSeconds > 0 {
		go func() {
			for {
				time.Sleep(5 * time.Second)
				if time.Since(lastMessageTime) > time.Duration(config.IdleTimeoutSeconds)*time.Second {
					log.Printf("No messages received for %d seconds. Exiting.", config.IdleTimeoutSeconds)
					sourceClient.Disconnect(250)
					targetClient.Disconnect(250)
					os.Exit(0)
				}
			}
		}()
	}

	if config.RunMode == "once" {
		log.Println("Run mode is 'once'. Waiting for a single message...")
		time.Sleep(5 * time.Second)
		log.Println("Exiting after processing initial messages.")
		sourceClient.Disconnect(250)
		targetClient.Disconnect(250)
		os.Exit(0)
	}

	log.Println("Waiting for messages (daemon mode)...")
	select {}
}
