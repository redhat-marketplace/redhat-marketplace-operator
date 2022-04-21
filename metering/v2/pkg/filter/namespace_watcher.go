package filter

import (
	"sync"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NamespaceWatcher struct {
	namespaces map[client.ObjectKey]interface{}
	watches    []chan interface{}
	log        logr.Logger

	mu           sync.RWMutex
	watchesMutex sync.Mutex
}

func ProvideNamespaceWatcher(
	log logr.Logger,
) *NamespaceWatcher {
	return &NamespaceWatcher{
		namespaces: make(map[client.ObjectKey]interface{}),
		watches:    make([]chan interface{}, 0, 0),
		log:        log.WithName("namespace-watcher"),
	}
}

func (n *NamespaceWatcher) alert() {
	n.watchesMutex.Lock()
	defer n.watchesMutex.Unlock()

	for _, c := range n.watches {
		c <- true
	}
}

func (n *NamespaceWatcher) RegisterWatch(in chan interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.watches = append(n.watches, in)
	return nil
}

func (n *NamespaceWatcher) addNamespace(in client.ObjectKey) (alert bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	before := n.listNamespaces()

	if _, ok := n.namespaces[in]; !ok {
		n.namespaces[in] = nil

		after := n.listNamespaces()

		if !alert && len(before) != len(after) {
			alert = true
		}
	}

	return
}

func (n *NamespaceWatcher) AddNamespace(in client.ObjectKey) {
	alert := n.addNamespace(in)
	if alert {
		n.alert()
	}
}

func (n *NamespaceWatcher) removeNamespace(in client.ObjectKey) (alert bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	before := n.listNamespaces()

	if _, ok := n.namespaces[in]; ok {
		delete(n.namespaces, in)

		after := n.listNamespaces()

		if !alert && len(before) != len(after) {
			alert = true
		}
	}

	return
}

func (n *NamespaceWatcher) RemoveNamespace(in client.ObjectKey) {
	alert := n.removeNamespace(in)
	if alert {
		n.alert()
	}
}

func (n *NamespaceWatcher) listNamespaces() []string {
	nses := map[string]interface{}{}

	for k := range n.namespaces {
		nses[k.Namespace] = nil
	}

	nsesSlice := []string{}

	for k := range nses {
		nsesSlice = append(nsesSlice, k)
	}

	return nsesSlice
}

func (n *NamespaceWatcher) Get() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	resp := n.listNamespaces()
	n.log.Info("list namespaces", "namespaces", resp)
	return resp
}
