package main

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/sync/errgroup"
)

func publishFieldsToMQTT(client mqtt.Client, cfg Config, topicPrefix string, fields map[string]any) error {
	timeout := time.Duration(cfg.MQTT.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 3 * time.Second // default timeout
	}

	eg := errgroup.Group{}
	for fieldName, value := range fields {
		topic := fmt.Sprintf("%s/%s/%s/%s", cfg.MQTT.TopicRoot, cfg.ThermostatID, topicPrefix, fieldName)
		v := value
		eg.Go(func() error {
			return publishToMQTT(client, topic, v, timeout)
		})
	}
	return eg.Wait()
}

func publishToMQTT(client mqtt.Client, topic string, value any, timeout time.Duration) error {
	token := client.Publish(topic, 0, false, fmt.Sprintf("%v", value))
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("timeout publishing to MQTT topic '%s'", topic)
	}
	if token.Error() != nil {
		return fmt.Errorf("error publishing to MQTT topic '%s': %v", topic, token.Error())
	}
	return nil
}
