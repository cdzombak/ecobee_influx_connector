package main

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/sync/errgroup"
)

func publishFieldsToMQTT(client mqtt.Client, cfg Config, topicPrefix string, fields map[string]any) error {
	eg := errgroup.Group{}
	for fieldName, value := range fields {
		topic := fmt.Sprintf("%s/%s/%s/%s", cfg.MQTT.TopicRoot, cfg.ThermostatID, topicPrefix, fieldName)
		eg.Go(func() error {
			return publishToMQTT(client, topic, value)
		})
	}
	return eg.Wait()
}

func publishToMQTT(client mqtt.Client, topic string, value any) error {
	token := client.Publish(topic, 0, false, fmt.Sprintf("%v", value))
	// TODO(cdzombak): make timeout configurable
	if !token.WaitTimeout(3 * time.Second) {
		return fmt.Errorf("timeout publishing to MQTT topic '%s'", topic)
	}
	if token.Error() != nil {
		return fmt.Errorf("error publishing to MQTT topic '%s': %v", topic, token.Error())
	}
	return nil
}
