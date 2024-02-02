// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type mockClusterNameProvider struct {
}

func (m mockClusterNameProvider) GetClusterName() string {
	return "cluster-name"
}

type mockConsumer struct {
	t                *testing.T
	up               *bool
	httpConnected    *bool
	relabeled        *bool
	rpcDurationTotal *bool
}

func (m mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m mockConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	assert.Equal(m.t, 1, md.ResourceMetrics().Len())

	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)
		if metric.Name() == "http_connected_total" {
			assert.Equal(m.t, float64(15), metric.Sum().DataPoints().At(0).DoubleValue())
			*m.httpConnected = true

			_, relabeled := metric.Sum().DataPoints().At(0).Attributes().Get("kubernetes_port")
			_, originalLabel := metric.Sum().DataPoints().At(0).Attributes().Get("port")
			*m.relabeled = relabeled && !originalLabel
		}
		if metric.Name() == "rpc_duration_total" {
			*m.rpcDurationTotal = true
		}
		if metric.Name() == "up" {
			assert.Equal(m.t, float64(1), metric.Gauge().DataPoints().At(0).DoubleValue())
			*m.up = true
		}
	}

	return nil
}

func TestNewDcgmScraperBadInputs(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	tests := []DcgmScraperOpts{
		{
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Consumer:            mockConsumer{},
			Host:                componenttest.NewNopHost(),
			ClusterNameProvider: mockClusterNameProvider{},
		},
		{
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Consumer:            nil,
			Host:                componenttest.NewNopHost(),
			ClusterNameProvider: mockClusterNameProvider{},
		},
		{
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Consumer:            mockConsumer{},
			Host:                nil,
			ClusterNameProvider: mockClusterNameProvider{},
		},
		{
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Consumer:            mockConsumer{},
			Host:                componenttest.NewNopHost(),
			ClusterNameProvider: nil,
		},
	}

	for _, tt := range tests {
		scraper, err := NewDcgmScraper(tt)

		assert.Error(t, err)
		assert.Nil(t, scraper)
	}
}

// skipping for now as helper functions are under a different package
//func TestNewDcgmScraperEndToEnd(t *testing.T) {
//
//	upPtr := false
//	httpPtr := false
//	relabeledPtr := false
//	rpcDurationTotalPtr := false
//
//	consumer := mockConsumer{
//		t:                t,
//		up:               &upPtr,
//		httpConnected:    &httpPtr,
//		relabeled:        &relabeledPtr,
//		rpcDurationTotal: &rpcDurationTotalPtr,
//	}
//
//	settings := componenttest.NewNopTelemetrySettings()
//	settings.Logger, _ = zap.NewDevelopment()
//
//	scraper, err := NewDcgmScraper(DcgmScraperOpts{
//		Ctx:                 context.TODO(),
//		TelemetrySettings:   settings,
//		Consumer:            mockConsumer{},
//		Host:                componenttest.NewNopHost(),
//		ClusterNameProvider: mockClusterNameProvider{},
//	})
//	assert.NoError(t, err)
//	assert.Equal(t, mockClusterNameProvider{}, scraper.clusterNameProvider)
//
//	// build up a new PR
//	promFactory := prometheusreceiver.NewFactory()
//
//	targets := []*testData{
//		{
//			name: "prometheus",
//			pages: []mockPrometheusResponse{
//				{code: 200, data: renameMetric},
//			},
//		},
//	}
//	mp, cfg, err := setupMockPrometheus(targets...)
//	assert.NoError(t, err)
//
//	split := strings.Split(mp.srv.URL, "http://")
//
//	scrapeConfig := &config.ScrapeConfig{
//		HTTPClientConfig: configutil.HTTPClientConfig{
//			TLSConfig: configutil.TLSConfig{
//				InsecureSkipVerify: true,
//			},
//		},
//		ScrapeInterval:  cfg.ScrapeConfigs[0].ScrapeInterval,
//		ScrapeTimeout:   cfg.ScrapeConfigs[0].ScrapeInterval,
//		JobName:         fmt.Sprintf("%s/%s", jobName, cfg.ScrapeConfigs[0].MetricsPath),
//		HonorTimestamps: true,
//		Scheme:          "http",
//		MetricsPath:     cfg.ScrapeConfigs[0].MetricsPath,
//		ServiceDiscoveryConfigs: discovery.Configs{
//			&discovery.StaticConfig{
//				{
//					Targets: []model.LabelSet{
//						{
//							model.AddressLabel: model.LabelValue(split[1]),
//							"ClusterName":      model.LabelValue("test_cluster_name"),
//							"Version":          model.LabelValue("0"),
//							"Sources":          model.LabelValue("[\"apiserver\"]"),
//							"NodeName":         model.LabelValue("test"),
//							"Type":             model.LabelValue("ControlPlane"),
//						},
//					},
//				},
//			},
//		},
//		MetricRelabelConfigs: []*relabel.Config{
//			{
//				// allow list filter for the control plane metrics we care about
//				SourceLabels: model.LabelNames{"__name__"},
//				Regex:        relabel.MustNewRegexp("http_connected_total"),
//				Action:       relabel.Keep,
//			},
//			{
//				// type conflicts with the log Type in the container insights output format
//				Regex:       relabel.MustNewRegexp("^port$"),
//				Replacement: "kubernetes_port",
//				Action:      relabel.LabelMap,
//			},
//			{
//				Regex:  relabel.MustNewRegexp("^port"),
//				Action: relabel.LabelDrop,
//			},
//		},
//	}
//
//	promConfig := prometheusreceiver.Config{
//		PrometheusConfig: &config.Config{
//			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
//		},
//	}
//
//	// replace the prom receiver
//	params := receiver.CreateSettings{
//		TelemetrySettings: scraper.settings,
//	}
//	scraper.prometheusReceiver, err = promFactory.CreateMetricsReceiver(scraper.ctx, params, &promConfig, consumer)
//	assert.NoError(t, err)
//	assert.NotNil(t, mp)
//	defer mp.Close()
//
//	// perform a single scrape, this will kick off the scraper process for additional scrapes
//	scraper.GetMetrics()
//
//	t.Cleanup(func() {
//		scraper.Shutdown()
//	})
//
//	// wait for 2 scrapes, one initiated by us, another by the new scraper process
//	mp.wg.Wait()
//	mp.wg.Wait()
//
//	assert.True(t, *consumer.up)
//	assert.True(t, *consumer.httpConnected)
//	assert.True(t, *consumer.relabeled)
//	assert.False(t, *consumer.rpcDurationTotal) // this will get filtered out by our metric relabel config
//}

func TestDcgmScraperJobName(t *testing.T) {
	// needs to start with containerInsights
	assert.True(t, strings.HasPrefix(jobName, "containerInsightsDCGMExporterScraper"))
}
