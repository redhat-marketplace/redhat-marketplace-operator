package runnables

import (
	"context"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type PodMonitor struct {
	logger     logr.Logger
	cc         ClientCommandRunner
	client     kubernetes.Interface
	config     PodMonitorConfig
	deletePods deleteQueue
}

type PodMonitorConfig struct {
	Namespace string
	RetryTime time.Duration
}

func NewPodMonitor(
	logger logr.Logger,
	client kubernetes.Interface,
	cc ClientCommandRunner,
	config PodMonitorConfig,
) *PodMonitor {
	return &PodMonitor{
		logger: logger,
		cc:     cc,
		config: config,
		client: client,
	}
}

func (a *PodMonitor) NeedLeaderElection() bool {
	return true
}

func (a *PodMonitor) Start(stop <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go a.Run(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-stop:
			cancel()
			return nil
		}
	}
}

func (a *PodMonitor) Run(ctx context.Context) error {
	log := a.logger.WithValues("func", "podMonitor")

	podClient := a.client.CoreV1().Pods(a.config.Namespace)
	podWatcher, err := podClient.Watch(ctx, v1.ListOptions{})

	if err != nil {
		log.Error(err, "error on watch")
		return err
	}

	defer podWatcher.Stop()

	a.deletePods = deleteQueue{
		timers:        []*deletePod{},
		podClient:     podClient,
		expiredTimers: make(chan *deletePod),
		logger:        a.logger,
	}

	deletePodsContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.deletePods.Run(deletePodsContext)

	for {
		select {
		case evt := <-podWatcher.ResultChan():
			switch evt.Type {
			case watch.Error:
				err := errors.NewWithDetails("error reading channel", "err", evt.Object)
				log.Error(err, "error on watch")
				return err
			case watch.Modified:
				pod, ok := evt.Object.(*corev1.Pod)

				if !ok {
					continue
				}

				for _, status := range pod.Status.ContainerStatuses {
					if status.Name == "authcheck" && !status.Ready {
						if pod.Spec.RestartPolicy != corev1.RestartPolicyNever && !(status.RestartCount > 2) {
							break
						}

						a.deletePods.Add(ctx, &deletePod{
							pod:   pod,
							Timer: time.NewTimer(wait.Jitter(5*time.Second, 1.0)),
						})

						break
					}
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

type deletePod struct {
	*time.Timer
	pod *corev1.Pod
}

type deleteQueue struct {
	sync.Mutex

	expiredTimers chan *deletePod
	podClient     typedcorev1.PodInterface
	timers        []*deletePod
	logger        logr.Logger
}

func (d *deleteQueue) Add(ctx context.Context, pod *deletePod) {
	d.Lock()
	defer d.Unlock()

	// prevent dupes
	for _, timer := range d.timers {
		if timer.pod.UID == pod.pod.UID {
			return
		}
	}

	d.timers = append(d.timers, pod)
	go func() {
		localPod := pod
		select {
		case <-localPod.C:
			d.expiredTimers <- localPod
		case <-ctx.Done():
			return
		}
	}()
}

func (d *deleteQueue) delete(delete *deletePod) {
	d.Lock()
	defer d.Unlock()
	d.logger.Info("deleting pod", "name", delete.pod.Name)
	d.podClient.Delete(context.TODO(), delete.pod.Name, v1.DeleteOptions{})

	timers := d.timers[:0]
	for _, oldtimer := range d.timers {
		if oldtimer.pod.UID == delete.pod.UID {
			continue
		}

		timers = append(timers, oldtimer)
	}

	d.timers = timers
}

func (d *deleteQueue) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case timer := <-d.expiredTimers:
			d.delete(timer)
		}
	}
}
