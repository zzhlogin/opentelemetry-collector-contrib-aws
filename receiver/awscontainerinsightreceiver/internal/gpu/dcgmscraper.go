// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"context"
	"errors"
	"fmt"
	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/model/relabel"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

const (
	caFile             = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	collectionInterval = 60 * time.Second
	jobName            = "containerInsightsDCGMExporterScraper"
)

type DcgmScraper struct {
	ctx                 context.Context
	settings            component.TelemetrySettings
	host                component.Host
	clusterNameProvider clusterNameProvider
	prometheusReceiver  receiver.Metrics
	running             bool
}

type DcgmScraperOpts struct {
	Ctx                 context.Context
	TelemetrySettings   component.TelemetrySettings
	Consumer            consumer.Metrics
	Host                component.Host
	ClusterNameProvider clusterNameProvider
	Endpoint            string
	BearerToken         string
}

type clusterNameProvider interface {
	GetClusterName() string
}

type printConsumer struct {
}

func (pc printConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (pc printConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	if md.ResourceMetrics().Len() > 0 {
		scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		metrics := ""
		for i := 0; i < scopeMetrics.Len(); i++ {
			metric := scopeMetrics.At(i)
			metrics = fmt.Sprintf("%s:__:%s", metrics, metric.Name())
		}
		return errors.New(metrics)
	}

	return nil
}

func NewDcgmScraper(opts DcgmScraperOpts) (*DcgmScraper, error) {
	if opts.Consumer == nil {
		return nil, errors.New("consumer cannot be nil")
	}
	if opts.Host == nil {
		return nil, errors.New("host cannot be nil")
	}
	if opts.ClusterNameProvider == nil {
		return nil, errors.New("cluster name provider cannot be nil")
	}
	if opts.Endpoint == "" {
		return nil, errors.New("k8s api server endpoint cannot be nil")
	}

	//apiserverUrl, _ := url.Parse(fmt.Sprintf("https://%s", opts.Endpoint))

	scrapeConfig := &config.ScrapeConfig{
		HTTPClientConfig: configutil.HTTPClientConfig{
			TLSConfig: configutil.TLSConfig{
				CAFile:             caFile,
				InsecureSkipVerify: true,
			},
		},
		ScrapeInterval: model.Duration(collectionInterval),
		ScrapeTimeout:  model.Duration(collectionInterval),
		JobName:        jobName,
		Scheme:         "http",
		MetricsPath:    "/metrics",
		ServiceDiscoveryConfigs: discovery.Configs{
			&kubernetes.SDConfig{
				Role: kubernetes.RolePod,
				Selectors: []kubernetes.SelectorConfig{
					{
						Role:  kubernetes.RolePod,
						Label: "app=nvidia-dcgm-exporter",
					},
				},
			},
		},
		//ServiceDiscoveryConfigs: discovery.Configs{
		//	&discovery.StaticConfig{
		//		{
		//			Targets: []model.LabelSet{
		//				{
		//					model.AddressLabel: model.LabelValue("192.168.72.110:9400"),
		//				},
		//			},
		//		},
		//	},
		//	//192.168.72.110:9400
		//	//&kubernetes.SDConfig{
		//	//	APIServer: configutil.URL{
		//	//		URL: apiserverUrl,
		//	//	},
		//	//	HTTPClientConfig: configutil.HTTPClientConfig{
		//	//		TLSConfig: configutil.TLSConfig{
		//	//			CAFile:             caFile,
		//	//			InsecureSkipVerify: false,
		//	//		},
		//	//		Authorization: &configutil.Authorization{
		//	//			Type: "Bearer",
		//	//		},
		//	//		BearerToken: configutil.Secret(opts.BearerToken),
		//	//	},
		//	//	Role: kubernetes.RolePod,
		//	//	//Role: kubernetes.RoleService,
		//	//	//NamespaceDiscovery: kubernetes.NamespaceDiscovery{
		//	//	//	IncludeOwnNamespace: true,
		//	//	//},
		//	//	Selectors: []kubernetes.SelectorConfig{
		//	//		{
		//	//			Role:  kubernetes.RolePod,
		//	//			Label: "k8s-app=dcgm-exporter",
		//	//		},
		//	//		//{
		//	//		//	Role:  kubernetes.RoleService,
		//	//		//	Label: "k8s-app=dcgm-exporter-service",
		//	//		//},
		//	//	},
		//	//},
		//},
		MetricRelabelConfigs: []*relabel.Config{
			//{
			//	SourceLabels: model.LabelNames{"__meta_kubernetes_pod_container_name"},
			//	Regex:        relabel.MustNewRegexp(".*dcgm.*"),
			//	Action:       relabel.Keep,
			//},
		},
	}

	promConfig := prometheusreceiver.Config{
		PrometheusConfig: &config.Config{
			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
		},
	}

	params := receiver.CreateSettings{
		TelemetrySettings: opts.TelemetrySettings,
	}

	promFactory := prometheusreceiver.NewFactory()
	promReceiver, err := promFactory.CreateMetricsReceiver(opts.Ctx, params, &promConfig, printConsumer{})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus receiver: %w", err)
	}

	return &DcgmScraper{
		ctx:                 opts.Ctx,
		settings:            opts.TelemetrySettings,
		host:                opts.Host,
		clusterNameProvider: opts.ClusterNameProvider,
		prometheusReceiver:  promReceiver,
	}, nil
}

func (ds *DcgmScraper) GetMetrics() []pmetric.Metrics {
	//client := &http.Client{}
	//req, err := http.NewRequest("GET", "http://dcgm-exporter-service:9400/metrics", nil)
	////req.SetBasicAuth("cwagent", "cwagent-dcgm-exporter")
	//resp, err := client.Do(req)
	//if err != nil {
	//	ds.settings.Logger.Info(fmt.Sprintf("======================== HTTP/Get Error: %s", err))
	//}
	//body, err := io.ReadAll(resp.Body)
	//if err != nil {
	//	ds.settings.Logger.Info(fmt.Sprintf("======================== ReadAll Error: %s", err))
	//}
	//ds.settings.Logger.Info(fmt.Sprintf("======================== Metric Body: %s", string(body)))

	// This method will never return metrics because the metrics are collected by the scraper.
	// This method will ensure the scraper is running
	if !ds.running {
		ds.settings.Logger.Info("The scraper is not running, starting up the scraper")
		err := ds.prometheusReceiver.Start(ds.ctx, ds.host)
		if err != nil {
			ds.settings.Logger.Error("Unable to start PrometheusReceiver", zap.Error(err))
		}
		ds.running = err == nil
	}
	return nil
}

func (ds *DcgmScraper) Shutdown() {
	if ds.running {
		err := ds.prometheusReceiver.Shutdown(ds.ctx)
		if err != nil {
			ds.settings.Logger.Error("Unable to shutdown PrometheusReceiver", zap.Error(err))
		}
		ds.running = false
	}
}
