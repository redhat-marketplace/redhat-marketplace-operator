package engine

import (
	"sync"
)

type NamespaceWatcher struct {
	namespaces map[string]interface{}
	watches    []chan interface{}

	mu sync.RWMutex
}

func ProvideNamespaceWatcher() *NamespaceWatcher {
	return &NamespaceWatcher{}
}

func (n *NamespaceWatcher) alert() {
	for _, c := range n.watches {
		c <- true
	}
}

func (n *NamespaceWatcher) AddNamespace(ns string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.namespaces[ns] = nil
	n.alert()

	return nil
}

func (n *NamespaceWatcher) RegisterWatch(in chan interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.watches = append(n.watches, in)
	return nil
}

func (n *NamespaceWatcher) RemoveNamespace(ns string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.namespaces, ns)
	n.alert()

	return nil
}

func (n *NamespaceWatcher) Get() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	nses := []string{}

	for k := range n.namespaces {
		nses = append(nses, k)
	}

	return nses
}
