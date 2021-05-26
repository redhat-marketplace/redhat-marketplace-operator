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
	"time"

	"github.com/go-co-op/gocron"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SchedulerConfig struct {
	Log logr.Logger
	Fs  *database.Database
	Job *Config
}

type Config struct {
	CleanAfter
	PurgeAfter
}
type CleanAfter struct {
	Cron string
	Days int
}

type PurgeAfter struct {
	Cron string
	Days int
}

// Schedule creates and returns gocron.scheduler
func (sfg *SchedulerConfig) schedule() *gocron.Scheduler {
	if sfg.Job == nil {
		return nil
	}
	sfg.Log.Info("creating scheduler with jobs", "clean after", sfg.Job.CleanAfter, "purge after", sfg.Job.PurgeAfter)

	s := gocron.NewScheduler(time.UTC)
	s.SetMaxConcurrentJobs(1, gocron.WaitMode)

	if sfg.Job.CleanAfter.Cron != "" && sfg.Job.CleanAfter.Days != 0 {
		_, err := s.CronWithSeconds(sfg.Job.CleanAfter.Cron).Do(func() {
			sfg.handler(sfg.Job.CleanAfter.Days, false)
		})

		if err != nil {
			sfg.Log.Error(err, "error creating job")
			return nil
		}
	}

	if sfg.Job.PurgeAfter.Cron != "" && sfg.Job.PurgeAfter.Days != 0 {
		_, err := s.CronWithSeconds(sfg.Job.PurgeAfter.Cron).Do(func() {
			sfg.handler(sfg.Job.PurgeAfter.Days, true)
		})
		if err != nil {
			sfg.Log.Error(err, "error creating job")
			return nil
		}
	}

	if len(s.Jobs()) == 0 {
		return nil
	}

	return s
}

// handler is jobFuncthat that should be called every time the Job runs
func (sfg *SchedulerConfig) handler(before int, purge bool) error {
	sfg.Log.Info("Job", "time", time.Now().Unix(), "diff", before, "purge", purge)

	now := time.Now()
	t1 := now.AddDate(0, 0, -before).Unix()
	t := &timestamppb.Timestamp{Seconds: t1}
	sfg.Log.Info("cleaning before", "date", t1)

	fileId, err := sfg.Fs.CleanTombstones(t, purge)
	if err != nil {
		sfg.Log.Error(err, "")
	}

	sfg.Log.Info("FileId", "ids", fileId)
	return nil
}

// Start starts all job for passed scheduler
func (sfg *SchedulerConfig) Start() {
	s := sfg.schedule()
	if s != nil {
		sfg.Log.Info("starting scheduler")
		s.StartAsync()
	} else {
		sfg.Log.Info("no scheduler to start")
	}
}
