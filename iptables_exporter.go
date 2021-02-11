// Copyright 2018 RetailNext, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/steigr/iptables_exporter/iptables"
	"gopkg.in/alecthomas/kingpin.v2"
)

type collector struct {
	capture *regexp.Regexp
}

type ruleCounter map[string]*ruleValues

type ruleValues struct {
	bytes   float64
	packets float64
}

var (
	scrapeDurationDesc = prometheus.NewDesc(
		"iptables_scrape_duration_seconds",
		"iptables_exporter: Duration of scraping iptables.",
		nil,
		nil,
	)

	scrapeSuccessDesc = prometheus.NewDesc(
		"iptables_scrape_success",
		"iptables_exporter: Whether scraping iptables succeeded.",
		nil,
		nil,
	)

	defaultBytesDesc = prometheus.NewDesc(
		"iptables_default_bytes_total",
		"iptables_exporter: Total bytes matching a chain's default policy.",
		[]string{"table", "chain", "policy"},
		nil,
	)

	defaultPacketsDesc = prometheus.NewDesc(
		"iptables_default_packets_total",
		"iptables_exporter: Total packets matching a chain's default policy.",
		[]string{"table", "chain", "policy"},
		nil,
	)

	ruleBytesDesc = prometheus.NewDesc(
		"iptables_rule_bytes_total",
		"iptables_exporter: Total bytes matching a rule.",
		[]string{"table", "chain", "rule"},
		nil,
	)

	rulePacketsDesc = prometheus.NewDesc(
		"iptables_rule_packets_total",
		"iptables_exporter: Total packets matching a rule.",
		[]string{"table", "chain", "rule"},
		nil,
	)
)

func NewCollector(captureRE string) collector {
	// Let regexp.MustCompile panic if regex is not valid
	return collector{
		capture: regexp.MustCompile(captureRE),
	}
}

func (c *collector) Describe(descChan chan<- *prometheus.Desc) {
	descChan <- scrapeDurationDesc
	descChan <- scrapeSuccessDesc
	descChan <- defaultBytesDesc
	descChan <- defaultPacketsDesc
	descChan <- ruleBytesDesc
	descChan <- rulePacketsDesc
}

func (c *collector) Collect(metricChan chan<- prometheus.Metric) {
	start := time.Now()
	tables, err := iptables.GetTables(c.capture)
	duration := time.Since(start)
	if err == nil && len(tables) == 0 {
		err = errors.New("no output from iptables-save; this is probably due to insufficient permissions")
	}
	metricChan <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds())
	if err != nil {
		metricChan <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, 0)
		log.Error(err)
		return
	}
	metricChan <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, 1)

	for tableName, table := range tables {
		for chainName, chain := range table {
			metricChan <- prometheus.MustNewConstMetric(
				defaultPacketsDesc,
				prometheus.CounterValue,
				float64(chain.Packets),
				tableName,
				chainName,
				chain.Policy,
			)
			metricChan <- prometheus.MustNewConstMetric(
				defaultBytesDesc,
				prometheus.CounterValue,
				float64(chain.Bytes),
				tableName,
				chainName,
				chain.Policy,
			)
			// Dedup rules if they have the same identifier
			rulesCounters := make(ruleCounter)
			for _, rule := range chain.Rules {
				if _, ok := rulesCounters[rule.Rule]; ok {
					log.Debugf("Merging counters for %s in chain %s[%s]", rule.Rule, chainName, tableName)
					rulesCounters[rule.Rule].bytes += float64(rule.Bytes)
					rulesCounters[rule.Rule].packets += float64(rule.Packets)
				} else {
					rulesCounters[rule.Rule] = &ruleValues{
						bytes:   float64(rule.Bytes),
						packets: float64(rule.Packets),
					}
				}
			}
			for ruleName, ruleData := range rulesCounters {
				metricChan <- prometheus.MustNewConstMetric(
					rulePacketsDesc,
					prometheus.CounterValue,
					ruleData.packets,
					tableName,
					chainName,
					ruleName,
				)
				metricChan <- prometheus.MustNewConstMetric(
					ruleBytesDesc,
					prometheus.CounterValue,
					ruleData.bytes,
					tableName,
					chainName,
					ruleName,
				)
			}
		}
	}
}

func main() {
	// Adapted from github.com/prometheus/node_exporter

	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address on which to expose metrics and web interface.").Default(":9455").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		captureRE     = kingpin.Flag("iptables.capture-re", "Regular expression used to export as 'rule' label desired bits from iptables rule").Default(`.*`).String()
	)

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("iptables_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting iptables_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	c := NewCollector(*captureRE)
	prometheus.MustRegister(&c)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>iptables exporter</title></head>
			<body>
			<h1>iptables exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})

	log.Infoln("Listening on", *listenAddress)
	err := http.ListenAndServe(*listenAddress, nil)
	if err != nil {
		log.Fatal(err)
	}
}
