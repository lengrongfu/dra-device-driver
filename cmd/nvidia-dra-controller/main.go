/*
 * Copyright (c) 2024 NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v2"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"

	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
	_ "k8s.io/component-base/metrics/prometheus/workqueue"  // register work queues in the default legacy registry

	"github.com/NVIDIA/k8s-dra-driver/internal/info"
	"github.com/NVIDIA/k8s-dra-driver/pkg/flags"
)

type Flags struct {
	kubeClientConfig flags.KubeClientConfig
	loggingConfig    *flags.LoggingConfig

	podName   string
	namespace string

	httpEndpoint string
	metricsPath  string
	profilePath  string

	deviceClasses sets.Set[string]
}

type Config struct {
	flags      *Flags
	clientSets flags.ClientSets
	mux        *http.ServeMux
}

func main() {
	if err := newApp().Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newApp() *cli.App {
	flags := &Flags{
		loggingConfig: flags.NewLoggingConfig(),
	}
	cliFlags := []cli.Flag{
		&cli.StringFlag{
			Name:        "pod-name",
			Usage:       "The name of the pod this controller is running in.",
			Required:    true,
			Destination: &flags.podName,
			EnvVars:     []string{"POD_NAME"},
		},
		&cli.StringFlag{
			Name:        "namespace",
			Usage:       "The namespace of the pod this controller is running in.",
			Value:       "default",
			Destination: &flags.namespace,
			EnvVars:     []string{"NAMESPACE"},
		},
		&cli.StringFlag{
			Category:    "HTTP server:",
			Name:        "http-endpoint",
			Usage:       "The TCP network `address` where the HTTP server for diagnostics, including pprof and metrics will listen (example: `:8080`). The default is the empty string, which means the server is disabled.",
			Destination: &flags.httpEndpoint,
			EnvVars:     []string{"HTTP_ENDPOINT"},
		},
		&cli.StringFlag{
			Category:    "HTTP server:",
			Name:        "metrics-path",
			Usage:       "The HTTP `path` where Prometheus metrics will be exposed, disabled if empty.",
			Value:       "/metrics",
			Destination: &flags.metricsPath,
			EnvVars:     []string{"METRICS_PATH"},
		},
		&cli.StringFlag{
			Category:    "HTTP server:",
			Name:        "pprof-path",
			Usage:       "The HTTP `path` where pprof profiling will be available, disabled if empty.",
			Destination: &flags.profilePath,
			EnvVars:     []string{"PPROF_PATH"},
		},
		&cli.StringSliceFlag{
			Name:    "device-classes",
			Usage:   "The supported set of DRA device classes",
			Value:   cli.NewStringSlice(GpuDeviceType, MigDeviceType, ImexChannelType),
			EnvVars: []string{"DEVICE_CLASSES"},
		},
	}

	cliFlags = append(cliFlags, flags.kubeClientConfig.Flags()...)
	cliFlags = append(cliFlags, flags.loggingConfig.Flags()...)

	app := &cli.App{
		Name:            "nvidia-dra-controller",
		Usage:           "nvidia-dra-controller implements a DRA driver controller for NVIDIA GPUs.",
		ArgsUsage:       " ",
		HideHelpCommand: true,
		Flags:           cliFlags,
		Before: func(c *cli.Context) error {
			if c.Args().Len() > 0 {
				return fmt.Errorf("arguments not supported: %v", c.Args().Slice())
			}
			return flags.loggingConfig.Apply()
		},
		Action: func(c *cli.Context) error {
			mux := http.NewServeMux()
			flags.deviceClasses = sets.New[string](c.StringSlice("device-classes")...)

			clientSets, err := flags.kubeClientConfig.NewClientSets()
			if err != nil {
				return fmt.Errorf("create client: %w", err)
			}

			config := &Config{
				mux:        mux,
				flags:      flags,
				clientSets: clientSets,
			}

			if flags.httpEndpoint != "" {
				err = SetupHTTPEndpoint(config)
				if err != nil {
					return fmt.Errorf("create http endpoint: %w", err)
				}
			}

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

			var imexManager *ImexManager
			ctx, cancel := context.WithCancel(c.Context)
			defer func() {
				cancel()
				if err := imexManager.Stop(); err != nil {
					klog.Errorf("Error stopping IMEX manager: %v", err)
				}
			}()

			if flags.deviceClasses.Has(ImexChannelType) {
				imexManager, err = StartIMEXManager(ctx, config)
				if err != nil {
					return fmt.Errorf("start IMEX manager: %w", err)
				}
			}

			<-sigs

			return nil
		},
		Version: info.GetVersionString(),
	}

	// We remove the -v alias for the version flag so as to not conflict with the -v flag used for klog.
	f, ok := cli.VersionFlag.(*cli.BoolFlag)
	if ok {
		f.Aliases = nil
	}

	return app
}

func SetupHTTPEndpoint(config *Config) error {
	if config.flags.metricsPath != "" {
		// To collect metrics data from the metric handler itself, we
		// let it register itself and then collect from that registry.
		reg := prometheus.NewRegistry()
		gatherers := prometheus.Gatherers{
			// Include Go runtime and process metrics:
			// https://github.com/kubernetes/kubernetes/blob/9780d88cb6a4b5b067256ecb4abf56892093ee87/staging/src/k8s.io/component-base/metrics/legacyregistry/registry.go#L46-L49
			legacyregistry.DefaultGatherer,
		}
		gatherers = append(gatherers, reg)

		actualPath := path.Join("/", config.flags.metricsPath)
		klog.InfoS("Starting metrics", "path", actualPath)
		// This is similar to k8s.io/component-base/metrics HandlerWithReset
		// except that we gather from multiple sources.
		config.mux.Handle(actualPath,
			promhttp.InstrumentMetricHandler(
				reg,
				promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})))
	}

	if config.flags.profilePath != "" {
		actualPath := path.Join("/", config.flags.profilePath)
		klog.InfoS("Starting profiling", "path", actualPath)
		config.mux.HandleFunc(actualPath, pprof.Index)
		config.mux.HandleFunc(path.Join(actualPath, "cmdline"), pprof.Cmdline)
		config.mux.HandleFunc(path.Join(actualPath, "profile"), pprof.Profile)
		config.mux.HandleFunc(path.Join(actualPath, "symbol"), pprof.Symbol)
		config.mux.HandleFunc(path.Join(actualPath, "trace"), pprof.Trace)
	}

	listener, err := net.Listen("tcp", config.flags.httpEndpoint)
	if err != nil {
		return fmt.Errorf("listen on HTTP endpoint: %w", err)
	}

	go func() {
		klog.InfoS("Starting HTTP server", "endpoint", config.flags.httpEndpoint)
		err := http.Serve(listener, config.mux)
		if err != nil {
			klog.ErrorS(err, "HTTP server failed")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
	}()

	return nil
}