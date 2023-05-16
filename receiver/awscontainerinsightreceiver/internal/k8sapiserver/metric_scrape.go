package k8sapiserver

import (
	"context"
	"fmt"
	"time"

	gokitLog "github.com/go-kit/log"
	httpconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/logging"
)

const (
	collectionEndpoint = "/metrics"
	bearerTokenPath    = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	collectionInterval = 15 * time.Second // TODO:
	collectionTimeout  = 7 * time.Second  // TODO
)

type metricScrape struct {
	scrapeManager    *scrape.Manager
	discoveryManager *discovery.Manager
	logger           *zap.Logger
	gokitlogger      gokitLog.Logger
	store            *storage.Appendable
	config           *config.Config
	cancel           context.CancelFunc
	context          context.Context
}

func NewMetricScrape(host string, logger *zap.Logger, store *storage.Appendable) (*metricScrape, error) {
	// TODO
	/* 2023-05-16T18:00:47.956Z        debug   scrape/scrape.go:1351   Scrape failed   {"kind": "receiver", "name": "awscontainerinsightreceiver", "data_type": "metrics", "scrape_pool": "k8sapiserver", "target": "https://ip-192-168-71-146.us-east-2.compute.internal:443/metrics", "error": "Get \"https://ip-192-168-71-146.us-east-2.compute.internal:443/metrics\": unable to read authorization credentials file /var/Run/secrets/kubernetes.io/serviceaccount/token: open /var/Run/secrets/kubernetes.io/serviceaccount/token: no such file or directory"}
	 */

	config := &config.Config{
		ScrapeConfigs: []*config.ScrapeConfig{
			{
				JobName: "k8sapiserver",

				ScrapeInterval: model.Duration(collectionInterval),
				ScrapeTimeout:  model.Duration(collectionTimeout),

				HonorLabels:     true,
				HonorTimestamps: true,

				MetricsPath: collectionEndpoint,
				Scheme:      "https",

				HTTPClientConfig: httpconfig.HTTPClientConfig{
					TLSConfig: httpconfig.TLSConfig{
						InsecureSkipVerify: true,
					},
					BearerTokenFile: bearerTokenPath,
					FollowRedirects: true,
				},

				ServiceDiscoveryConfigs: discovery.Configs{
					discovery.StaticConfig{
						{
							Targets: []model.LabelSet{
								{model.AddressLabel: model.LabelValue(host)},
							},
							Source: "0",
						},
					},
				},
			},
		},
	}

	// the prometheus scraper library uses gokit logging, we must adapt our zap logger to gokit
	gokitlogger := logging.NewZapToGokitLogAdapter(logger)

	ms := metricScrape{
		logger:      logger,
		gokitlogger: gokitlogger,
		store:       store,
		config:      config,
	}

	ms.context, ms.cancel = context.WithCancel(context.Background())

	ms.scrapeManager = scrape.NewManager(&scrape.Options{PassMetadataInContext: true}, gokitlogger, *store)
	ms.discoveryManager = discovery.NewManager(ms.context, gokitlogger)

	return &ms, nil
}

func (ms *metricScrape) Run() {
	ms.scrapeManager.ApplyConfig(ms.config)

	discoveryConfigs := make(map[string]discovery.Configs)
	discoveryConfigs["k8sapiserver"] = ms.config.ScrapeConfigs[0].ServiceDiscoveryConfigs
	ms.discoveryManager.ApplyConfig(discoveryConfigs)

	go ms.runDiscoveryManager()
	go ms.runScrapeManager()
}

func (ms *metricScrape) Shutdown(context.Context) error {
	if ms.cancel != nil {
		ms.cancel()
	}
	if ms.scrapeManager != nil {
		ms.scrapeManager.Stop()
	}
	return nil
}

func (ms *metricScrape) runDiscoveryManager() {
	ms.logger.Info("Starting k8sapiserver prometheus metrics discovery manager")
	if err := ms.discoveryManager.Run(); err != nil {
		ms.logger.Error("Discovery manager failed", zap.Error(err))
		return
	}

	select {
	case <-ms.context.Done():
		ms.logger.Info(fmt.Sprintf("Stopping k8sapiserver prometheus metrics discovery manager"))
		return
	default:
	}
}

func (ms *metricScrape) runScrapeManager() {
	// The scrape manager needs to wait for the configuration to be loaded before beginning
	// TODO <-r.configLoaded
	// TODO: is there anything to do here?
	ms.logger.Warn("Starting k8sapiserver prometheus metrics scrape manager")
	if err := ms.scrapeManager.Run(ms.discoveryManager.SyncCh()); err != nil {
		ms.logger.Error("Scrape manager failed", zap.Error(err))
		return
	}

	select {
	case <-ms.context.Done():
		ms.logger.Info(fmt.Sprintf("Stopping k8sapiserver prometheus metrics scrape manager"))
		return
	default:
	}
}
