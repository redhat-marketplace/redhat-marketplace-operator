package types

import "k8s.io/client-go/tools/cache"

type Stores []cache.Store
type PrimaryStore struct {
	cache.Store
}

var _ cache.Store = Stores{}

// Add adds the given object to the accumulator associated with the given object's key
func (stores Stores) Add(obj interface{}) error {
	for _, store := range stores {
		err := store.Add(obj)

		if err != nil {
			return err
		}
	}
	return nil
}

// Update updates the given object in the accumulator associated with the given object's key
func (stores Stores) Update(obj interface{}) error {
	for _, store := range stores {
		err := store.Update(obj)

		if err != nil {
			return err
		}
	}
	return nil

}

// Delete deletes the given object from the accumulator associated with the given object's key
func (stores Stores) Delete(obj interface{}) error {
	for _, store := range stores {
		err := store.Delete(obj)

		if err != nil {
			return err
		}
	}
	return nil

}

// List returns a list of all the currently non-empty accumulators
func (stores Stores) List() []interface{} {
	for _, store := range stores {
		if _, ok := store.(PrimaryStore); !ok {
			continue
		}

		err := store.List()

		if err != nil {
			return err
		}
	}
	return nil

}

// ListKeys returns a list of all the keys currently associated with non-empty accumulators
func (stores Stores) ListKeys() []string {
	for _, store := range stores {
		if _, ok := store.(PrimaryStore); !ok {
			continue
		}

		return store.ListKeys()
	}

	return nil
}

// Get returns the accumulator associated with the given object's key
func (stores Stores) Get(obj interface{}) (item interface{}, exists bool, err error) {
	for _, store := range stores {
		if _, ok := store.(PrimaryStore); !ok {
			continue
		}

		return store.Get(obj)
	}

	return nil, false, nil
}

// GetByKey returns the accumulator associated with the given key
func (stores Stores) GetByKey(key string) (item interface{}, exists bool, err error) {
	for _, store := range stores {
		if _, ok := store.(PrimaryStore); !ok {
			continue
		}

		return store.GetByKey(key)
	}

	return nil, false, nil
}

// Replace will delete the contents of the store, using instead the
// given list. Store takes ownership of the list, you should not reference
// it after calling this function.
func (stores Stores) Replace(data []interface{}, resourceVersion string) error {
	for _, store := range stores {
		err := store.Replace(data, resourceVersion)

		if err != nil {
			return err
		}
	}

	return nil
}

// Resync is meaningless in the terms appearing here but has
// meaning in some implementations that have non-trivial
// additional behavior (e.g., DeltaFIFO).
func (stores Stores) Resync() error {
	for _, store := range stores {
		err := store.Resync()

		if err != nil {
			return err
		}
	}
	return nil
}
