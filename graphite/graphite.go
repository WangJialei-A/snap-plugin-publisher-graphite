/*
http://www.apache.org/licenses/LICENSE-2.0.txt


Copyright 2016 Intel Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package graphite

import (
	"fmt"
	"strings"
	"time"

	"github.com/intelsdi-x/snap-plugin-lib-go/v1/plugin"
	"github.com/marpaia/graphite-golang"
	log "github.com/sirupsen/logrus"
)

const (
	Name    = "graphite"
	Version = 8
)

var (
	illegal     = "(), /{}"
	replacement = strings.NewReplacer(" ", "_",
		",", ";",
		"(", "[",
		")", "]",
		"/", "|",
		"{", "[",
		"}", "]")
)

type GraphitePublisher struct {
}

func (f *GraphitePublisher) Publish(metrics []plugin.Metric, cfg plugin.Config) error {

	logger := getLogger(cfg)
	logger.Debug("Publishing started")
	var tagsForPrefix []string

	logger.Debug("publishing %v metrics to %v", len(metrics), cfg)
	server, err := cfg.GetString("server")
	if err != nil {
		return err
	}
	port, err := cfg.GetInt("port")
	if err != nil {
		return err
	}
	tagConfigs, err := cfg.GetString("prefix_tags")
	if err == nil {
		tagsForPrefix = strings.Split(tagConfigs, ",")
	}
	pre, err := cfg.GetString("prefix")
	if err != nil {
		pre = ""
	}
	timeout, err := cfg.GetInt("timeout")
	if err != nil {
		timeout = 20
	}

	logger.Debug("Attempting to connect to %s:%d with timeout %d", server, port, timeout)
	gite := graphite.Graphite{
		Host:    server,
		Port:    int(port),
		Prefix:  pre,
		Timeout: time.Duration(timeout) * time.Second,
	}
	err = gite.Connect()
	if err != nil {
		logger.Errorf("Error Connecting to graphite at %s:%d. Error: %v", server, port, err)
		return fmt.Errorf("Error Connecting to graphite at %s:%d. Error: %v", server, port, err)
	}
	defer gite.Disconnect()
	logger.Debug("Connected to %s:%s successfully", server, port)

	giteMetrics := make([]graphite.Metric, len(metrics))
	for i, m := range metrics {
		key := strings.Join(m.Namespace.Strings(), ".")
		for _, tag := range tagsForPrefix {
			nextTag, ok := m.Tags[tag]
			if ok {
				key = nextTag + "." + key
			}
		}
		data := fmt.Sprintf("%v", m.Data)
		logger.Debug("Metric ready to send %s:%s", key, data)

		if strings.ContainsAny(key, illegal) {
			key := replacement.Replace(key)
			log.Info("Metric after replacement is %s", key)
		}

		giteMetrics[i] = graphite.NewMetric(key, data, m.Timestamp.Unix())
	}

	err = gite.SendMetrics(giteMetrics)
	if err != nil {
		logger.Errorf("Unable to send metrics. Error: %s", err)
		return fmt.Errorf("Unable to send metrics. Error: %s", err)
	}
	logger.Debug("Metrics sent to Graphite.")

	return nil
}

func (f *GraphitePublisher) GetConfigPolicy() (plugin.ConfigPolicy, error) {
	policy := plugin.NewConfigPolicy()

	policy.AddNewStringRule([]string{""}, "server", true)
	policy.AddNewIntRule([]string{""}, "port", false, plugin.SetDefaultInt(2003))
	policy.AddNewStringRule([]string{""}, "prefix_tags", false, plugin.SetDefaultString("plugin_running_on"))
	policy.AddNewStringRule([]string{""}, "prefix", false)
	policy.AddNewStringRule([]string{""}, "log-level", false)
	policy.AddNewIntRule([]string{""}, "timeout", false, plugin.SetDefaultInt(5))

	return *policy, nil
}

func getLogger(cfg plugin.Config) *log.Entry {
	logger := log.WithFields(log.Fields{
		"plugin-name":    Name,
		"plugin-version": Version,
		"plugin-type":    "publisher",
	})

	log.SetLevel(log.WarnLevel)

	levelValue, err := cfg.GetString("log-level")
	if err == nil {
		if level, err := log.ParseLevel(strings.ToLower(levelValue)); err == nil {
			log.SetLevel(level)
		} else {
			log.WithFields(log.Fields{
				"value":             strings.ToLower(levelValue),
				"acceptable values": "warn, error, debug, info",
			}).Warn("Invalid config value")
		}
	}
	return logger
}
