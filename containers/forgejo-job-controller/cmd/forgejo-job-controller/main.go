// Command forgejo-job-controller starts one Kubernetes pod per waiting Forgejo Actions job.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/config"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/forgejo"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/httpx"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/kube"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/logging"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/metrics"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/spawner"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "forgejo-job-controller:", err)
		os.Exit(1)
	}
}

func run() error {
	once := flag.Bool("once", false, "run one poll cycle and exit")
	flag.Parse()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	logging.Init(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = logging.WithTrace(ctx, logging.NewTraceID())
	log := logging.From(ctx)

	registry := &metrics.Registry{}
	calls := registry.NewHistogram("fjc_outbound_call_duration_seconds", "Duration of calls to other services.",
		[]float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}, "peer", "method", "status")

	kubeSettings := kube.Settings{APIURL: cfg.KubeAPIURL, TokenFile: cfg.KubeTokenFile, DryRun: cfg.KubeDryRun}
	kubeTransport, err := kube.NewTransport(kubeSettings)
	if err != nil {
		return err
	}
	kubeClient, err := kube.New(kubeSettings, &http.Client{
		Timeout:   30 * time.Second,
		Transport: httpx.Transport{Peer: "kubernetes-api", Base: kubeTransport, Duration: calls},
	})
	if err != nil {
		return err
	}
	forgejoClient := forgejo.New(cfg.ForgejoURL, cfg.ForgejoScope, cfg.ForgejoToken, &http.Client{
		Timeout:   30 * time.Second,
		Transport: httpx.Transport{Peer: "forgejo", Base: http.DefaultTransport, Duration: calls},
	})

	controller := spawner.New(cfg, forgejoClient, kubeClient, spawner.NewMetrics(registry))

	if *once {
		return controller.Tick(ctx)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", registry.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if !controller.Healthy() {
			http.Error(w, "no successful poll recently", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	server := &http.Server{Addr: cfg.Listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server stopped", "error", err.Error())
			stop()
		}
	}()

	log.Info("started", "scope", cfg.ForgejoScope, "namespace", cfg.Namespace, "max_jobs", cfg.MaxJobs,
		"poll_interval", cfg.PollInterval.String(), "dry_run", cfg.KubeDryRun)
	controller.Run(ctx)

	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	log.Info("stopping")
	return server.Shutdown(shutdown)
}
