// Copyright 2018 Palantir Technologies, Inc.
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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/v50/github"
	"github.com/gregjones/httpcache"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	// "github.com/rcrowley/go-metrics"

	// "github.com/rcrowley/go-metrics"
	"github.com/rs/zerolog"
)

type workflowJobHandler struct {
	githubapp.ClientCreator
}

var durationGauge *prometheus.GaugeVec

// Handle implements githubapp.EventHandler
func (h *workflowJobHandler) Handle(ctx context.Context, eventType string, deliveryID string, payload []byte) error {
	zerolog.Ctx(ctx).Debug().Msgf("Got event %s", eventType)
	var event github.WorkflowJobEvent

	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "failed to parse workflow_job event")
	}

	zerolog.Ctx(ctx).Debug().Msgf("Event action is %s", event.GetAction())
	if event.GetAction() != "completed" {
		return nil
	}

	job := event.GetWorkflowJob()
	dur := job.GetCompletedAt().Time.Sub(job.GetStartedAt().Time)

	job.GetWorkflowName()
	durationGauge.With(prometheus.Labels{
		"job_id":        fmt.Sprint(job.GetID()),
		"workflow_name": job.GetWorkflowName(),
		"job_name":      job.GetName(),
		"conclusion":    job.GetConclusion(),
	}).Set(dur.Seconds())

	return nil
}

// Handles implements githubapp.EventHandler
func (*workflowJobHandler) Handles() []string {
	return []string{"workflow_job"}
}

var _ githubapp.EventHandler = &workflowJobHandler{}

func main() {
	config, err := ReadConfig("config.yml")
	if err != nil {
		panic(err)
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	zerolog.DefaultContextLogger = &logger
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	durationGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hf",
			Subsystem: "github_actions",
			Name:      "job_duration",
			Help:      "TODO",
		},
		[]string{
			"job_id",
			"workflow_name",
			"job_name",
			// https://docs.github.com/webhooks-and-events/webhooks/webhook-events-and-payloads#workflow_job
			"conclusion",
		},
	)
	prometheus.MustRegister(durationGauge)

	cc, err := githubapp.NewDefaultCachingClientCreator(
		config.Github,
		githubapp.WithClientUserAgent("actions-metrics-app/0.0.1"),
		githubapp.WithClientTimeout(3*time.Second),
		githubapp.WithClientCaching(false, func() httpcache.Cache { return httpcache.NewMemoryCache() }),
	)
	if err != nil {
		panic(err)
	}

	workflowJobHandler := &workflowJobHandler{
		ClientCreator: cc,
	}

	webhookHandler := githubapp.NewDefaultEventDispatcher(config.Github, workflowJobHandler)

	http.Handle(githubapp.DefaultWebhookRoute, webhookHandler)
	http.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf("%s:%d", config.Server.Address, config.Server.Port)
	logger.Info().Msgf("Starting server on %s...", addr)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		panic(err)
	}
}
