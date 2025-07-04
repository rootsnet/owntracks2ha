package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
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
var logMutex sync.Mutex

func safeLogf(format string, v ...interface{}) {
	logMutex.Lock()
	defer logMutex.Unlock()
	log.Printf(format, v...)
}

func loadConfig(filename string) {
	file, err := os.ReadFile(filename)
	if err != nil {
		safeLogf("Failed to read config file: %v", err)
		os.Exit(1)
	}
	if err := yaml.Unmarshal(file, &config); err != nil {
		safeLogf("Failed to parse config file: %v", err)
		os.Exit(1)
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
	safeLogf("Received message from source topic: %s, payload: %s", msg.Topic(), string(msg.Payload()))

	var source SourceData
	if err := json.Unmarshal(msg.Payload(), &source); err != nil {
		safeLogf("Error parsing JSON: %v", err)
		return
	}

	if source.Lat == 0 || source.Lon == 0 {
		safeLogf("Invalid data received: missing latitude or longitude")
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
		safeLogf("No mapping found for topic: %s", subTopic)
		return
	}

	if config.Debug {
		raw, _ := json.MarshalIndent(source, "", "  ")
		conv, _ := json.MarshalIndent(converted, "", "  ")
		safeLogf("[DEBUG] Original data from %s:\n%s", subTopic, raw)
		safeLogf("[DEBUG] Converted data to %s:\n%s", pubTopic, conv)
	}

	payload, err := json.Marshal(converted)
	if err != nil {
		safeLogf("Error encoding JSON: %v", err)
		return
	}

	token := targetClient.Publish(pubTopic, byte(config.QoS), false, payload)
	token.Wait()
	if token.Error() != nil {
		safeLogf("Failed to publish message to %s: %v", pubTopic, token.Error())
	} else {
		safeLogf("Successfully published to %s: %s", pubTopic, payload)
	}
}

func main() {
	safeLogf("Loading configuration...")
	loadConfig("config/config.yaml")
	safeLogf("Configuration loaded successfully.")

	// Source broker setup
	sourceBroker := getBrokerURL(config.SourceBroker, config.SourcePort, config.UseTLS)
	safeLogf("Connecting to Source MQTT broker: %s", sourceBroker)
	sourceOpts := configureMQTTClientOptions(sourceBroker, "mqtt_converter", config.SourceUser, config.SourcePass, config.UseTLS)
	sourceOpts.SetDefaultPublishHandler(messageHandler)
	sourceClient := MQTT.NewClient(sourceOpts)
	token := sourceClient.Connect()
	if token.Wait() && token.Error() != nil {
		safeLogf("Source MQTT connection failed: %v", token.Error())
		os.Exit(1)
	}
	for !sourceClient.IsConnected() {
		safeLogf("Waiting for Source MQTT connection to establish...")
		time.Sleep(500 * time.Millisecond)
	}
	safeLogf("Connected to Source MQTT broker")

	// Target broker setup
	targetBroker := getBrokerURL(config.TargetBroker, config.TargetPort, config.UseTLS)
	safeLogf("Connecting to Target MQTT broker: %s", targetBroker)
	targetOpts := configureMQTTClientOptions(targetBroker, "mqtt_publisher", config.TargetUser, config.TargetPass, config.UseTLS)
	targetClient = MQTT.NewClient(targetOpts)
	token = targetClient.Connect()
	if token.Wait() && token.Error() != nil {
		safeLogf("Target MQTT connection failed: %v", token.Error())
		os.Exit(1)
	}
	for !targetClient.IsConnected() {
		safeLogf("Waiting for Target MQTT connection to establish...")
		time.Sleep(500 * time.Millisecond)
	}
	safeLogf("Connected to Target MQTT broker")

	// Subscribe to topics with retries
	for subTopic := range config.Mappings {
		safeLogf("Subscribing to topic: %s", subTopic)
		for attempt := 1; attempt <= 5; attempt++ {
			if !sourceClient.IsConnected() {
				safeLogf("Client not connected yet. Waiting to subscribe: %s", subTopic)
				time.Sleep(1 * time.Second)
				continue
			}
			token := sourceClient.Subscribe(subTopic, byte(config.QoS), nil)
			token.Wait()
			if token.Error() != nil {
				safeLogf("Subscription attempt %d failed for topic %s: %v", attempt, subTopic, token.Error())
				time.Sleep(1 * time.Second)
			} else {
				safeLogf("Successfully subscribed to topic: %s", subTopic)
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
					safeLogf("No messages received for %d seconds. Exiting.", config.IdleTimeoutSeconds)
					sourceClient.Disconnect(250)
					targetClient.Disconnect(250)
					os.Exit(0)
				}
			}
		}()
	}

	if config.RunMode == "once" {
		safeLogf("Run mode is 'once'. Waiting for a single message...")
		time.Sleep(5 * time.Second)
		safeLogf("Exiting after processing initial messages.")
		sourceClient.Disconnect(250)
		targetClient.Disconnect(250)
		os.Exit(0)
	}

	safeLogf("Waiting for messages (daemon mode)...")
	select {}
}
