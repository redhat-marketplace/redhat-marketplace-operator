// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scheduler

import (
	"context"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
)

type SchedulerConfig struct {
	Log            logr.Logger
	Fs             database.StoredFileStore
	IsLeader       func() (bool, error)
	CleanAfter     string
	CronExpression string
}

// handler cleans/purges files based on given time duration and purge flag
func (sfg *SchedulerConfig) handler() (int64, error) {
	isLeader, err := sfg.IsLeader()
	if err != nil {
		sfg.Log.Error(err, "error while verifying leader")
	}

	if !isLeader {
		return 0, nil
	}

	count, err := sfg.Fs.CleanTombstones(context.Background())
	if err != nil {
		sfg.Log.Error(err, "error while executing handler")
		return 0, err
	}

	sfg.Log.Info("result", "count", count)
	return count, nil
}

// StartScheduler starts all job(s) for created scheduler
func (sfg *SchedulerConfig) Start(ctx context.Context) error {
	s := gocron.NewScheduler(time.UTC)
	defer s.Stop()

	sfg.Log.Info("starting scheduler")
	s.StartAsync()

	job, err := s.Cron(sfg.CronExpression).Do(sfg.handler)
	if err != nil {
		sfg.Log.Error(err, "error while creating job")
		return err
	}
	job.SingletonMode()

	<-ctx.Done()
	sfg.Log.Info("stopping scheduler")
	return nil
}
