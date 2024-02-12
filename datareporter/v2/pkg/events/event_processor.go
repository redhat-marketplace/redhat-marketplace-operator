// Copyright 2023 IBM Corp.
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

package events

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	datareporterv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Must Start, Process and Send
type EventProcessorSender interface {
	Start(ctx context.Context) error
	Process(context.Context, Event) error
	Send(context.Context, UserManifestType) error
}

// Process events on the Event Channel and send when conditions are met
type ProcessorSender struct {
	log logr.Logger

	//retryCount    int
	digestersSize int
	EventChan     chan Event

	EventProcessorSender

	sendReadyChan chan UserManifestType

	eventAccumulator *EventAccumulator

	eventReporter *EventReporter

	config *Config

	client client.Client
}

func (p *ProcessorSender) Start(ctx context.Context, ready chan bool) error {
	ticker := time.NewTicker(p.config.MaxFlushTimeout.Duration)
	defer ticker.Stop()

	p.EventChan = make(chan Event)
	p.sendReadyChan = make(chan UserManifestType)

	p.eventAccumulator = &EventAccumulator{}

	// FlushAll intializes the map
	p.eventAccumulator.FlushAll()

	// Retries until an intial connection is successful
	dataService, err := p.provideDataService()
	if err != nil {
		return err
	}

	eventReporter, err := NewEventReporter(p.log, p.config, dataService)
	if err != nil {
		return err
	}
	p.eventReporter = eventReporter

	var processWaitGroup sync.WaitGroup
	var sendWaitGroup sync.WaitGroup

	processWaitGroup.Add(p.digestersSize)
	sendWaitGroup.Add(p.digestersSize)

	// Clear the DataReporterConfig error status before starting
	retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		dataReporterConfig := &v1alpha1.DataReporterConfig{}
		if err := p.client.Get(ctx, types.NamespacedName{Name: utils.DATAREPORTERCONFIG_NAME, Namespace: p.config.Namespace}, dataReporterConfig); err != nil {
			p.log.Error(err, "datareporterconfig resource not found, unable to update status")
			return err
		}
		if dataReporterConfig.Status.Conditions.RemoveCondition(status.ConditionType(datareporterv1alpha1.ConditionUploadFailure)) {
			p.log.Info("updating dataReporterConfig status")
			return p.client.Status().Update(context.TODO(), dataReporterConfig)
		}
		return nil
	})

	for i := 0; i < p.digestersSize; i++ {
		go func() {
			for event := range p.EventChan {
				localEvent := event
				if err := p.Process(ctx, localEvent); err != nil {
					p.log.Error(err, "error processes event")
				}
			}
			processWaitGroup.Done()
		}()
	}

	for i := 0; i < p.digestersSize; i++ {
		go func() {
			for key := range p.sendReadyChan {
				localKey := key
				if err := p.Send(ctx, localKey); err != nil {
					p.log.Error(err, "error sending event data")
					p.UpdateErrorStatus(ctx)
				}
			}
			sendWaitGroup.Done()
		}()
	}

	go func() {

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.log.V(4).Info("Timer expired. SendAll.")
				if err := p.SendAll(ctx); err != nil {
					p.log.Error(err, "error sending event data")
					p.UpdateErrorStatus(ctx)
				}
			}
		}
	}()

	// Ready to receive events
	close(ready)

	<-ctx.Done()
	p.log.Info("processor is shutting down")
	close(p.EventChan)
	close(p.sendReadyChan)
	processWaitGroup.Wait()
	sendWaitGroup.Wait()

	return nil
}

func (p *ProcessorSender) Process(ctx context.Context, event Event) error {

	len := p.eventAccumulator.Add(event)

	// If we are at event max, signal to send
	if len >= p.config.MaxEventEntries {
		p.sendReadyChan <- UserManifestType{User: event.User, ManifestType: event.ManifestType}
	}

	// If the map is at maximum size, signal to send

	return nil
}

// Send eventJsons for [user][manifestType]
func (p *ProcessorSender) Send(ctx context.Context, userManifestType UserManifestType) error {

	// flush entries for this key
	eventJsons := p.eventAccumulator.Flush(userManifestType)

	// Build and Send the report to dataService
	// There is a case where if an Userkey is removed, the metadata will no longer be available when the Report it sent
	metadata := p.config.UserConfigs.GetMetadata(userManifestType.User)
	if err := p.eventReporter.Report(metadata, userManifestType.ManifestType, eventJsons); err != nil {
		return err
	}
	p.log.Info("Sent Report")

	return nil
}

// Send all eventJsons for every [user][manifestType]
func (p *ProcessorSender) SendAll(ctx context.Context) error {

	// flush entire map [user][manifestType]eventJsons
	eventMap := p.eventAccumulator.FlushAll()

	for userManifestType, eventJsons := range eventMap {
		// Build and Send the report to dataService
		// There is a case where if an Userkey is removed, the metadata will no longer be available when the Report it sent
		metadata := p.config.UserConfigs.GetMetadata(userManifestType.User)
		if err := p.eventReporter.Report(metadata, userManifestType.ManifestType, eventJsons); err != nil {
			return err
		}
		p.log.Info("Sent Report")

	}

	return nil
}

func (p *ProcessorSender) provideDataServiceConfig() (*dataservice.DataServiceConfig, error) {

	var cert []byte
	var err error

	if p.config.DataServiceCertFile != "" {
		cert, err = os.ReadFile(p.config.DataServiceCertFile)
		if err != nil {
			return nil, err
		}
	}

	var serviceAccountToken = ""
	if p.config.DataServiceTokenFile != "" {
		content, err := os.ReadFile(p.config.DataServiceTokenFile)
		if err != nil {
			return nil, err
		}
		serviceAccountToken = string(content)
	}

	return &dataservice.DataServiceConfig{
		Address:          p.config.DataServiceURL.String(),
		DataServiceToken: serviceAccountToken,
		DataServiceCert:  cert,
		OutputPath:       p.config.OutputDirectory,
		CipherSuites:     p.config.CipherSuites,
		MinVersion:       p.config.MinVersion,
	}, nil
}

func (p *ProcessorSender) UpdateErrorStatus(ctx context.Context) error {

	retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		dataReporterConfig := &v1alpha1.DataReporterConfig{}
		if err := p.client.Get(ctx, types.NamespacedName{Name: utils.DATAREPORTERCONFIG_NAME, Namespace: p.config.Namespace}, dataReporterConfig); err != nil {
			p.log.Error(err, "datareporterconfig resource not found, unable to update status")
		} else {
			// report upload to data service failed, update status
			ok := dataReporterConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    datareporterv1alpha1.ConditionUploadFailure,
				Status:  corev1.ConditionTrue,
				Reason:  datareporterv1alpha1.ReasonUploadFailed,
				Message: "an error occured while uploading a report to data-service, check pod logs for more information",
			})
			if ok {
				return p.client.Status().Update(context.TODO(), dataReporterConfig)
			}
		}
		return nil
	})
	return nil
}

func (p *ProcessorSender) provideDataService() (*dataservice.DataService, error) {

	dataServiceConfig, err := p.provideDataServiceConfig()
	if err != nil {
		return nil, err
	}

	var dataService *dataservice.DataService
	for {
		// 60 second context timeout
		dataService, err = dataservice.NewDataService(dataServiceConfig)
		if err != nil {
			p.log.Error(err, "could not connect to data-service, retrying...")

			// Set Status on datareporterconfig
			retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				dataReporterConfig := &v1alpha1.DataReporterConfig{}
				if err := p.client.Get(context.TODO(), types.NamespacedName{Name: utils.DATAREPORTERCONFIG_NAME, Namespace: p.config.Namespace}, dataReporterConfig); err != nil {
					p.log.Error(err, "datareporterconfig resource not found, unable to update status")
				} else {
					if dataReporterConfig.Status.Conditions.SetCondition(status.Condition{
						Type:    datareporterv1alpha1.ConditionConnectionFailure,
						Status:  corev1.ConditionTrue,
						Reason:  datareporterv1alpha1.ReasonConnectionFailure,
						Message: "an error occured while attempting to initialize a connection to service/rhm-data-service",
					}) {
						return p.client.Status().Update(context.TODO(), dataReporterConfig)
					}
				}
				return nil
			})
		} else {
			// Remove Status on datareporterconfig and continue
			retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				dataReporterConfig := &v1alpha1.DataReporterConfig{}
				if err := p.client.Get(context.TODO(), types.NamespacedName{Name: utils.DATAREPORTERCONFIG_NAME, Namespace: p.config.Namespace}, dataReporterConfig); err != nil {
					p.log.Error(err, "datareporterconfig resource not found, unable to update status")
				} else {

					if dataReporterConfig.Status.Conditions.RemoveCondition(status.ConditionType(datareporterv1alpha1.ConditionConnectionFailure)) {
						return p.client.Status().Update(context.TODO(), dataReporterConfig)
					}
				}
				return nil
			})
			break
		}
	}
	return dataService, err
}
