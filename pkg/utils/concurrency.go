package utils

import (
	"time"

	"github.com/jpillora/backoff"
)

func Retry(f func() error, retry int) error {
	var err error

	jitter := &backoff.Backoff{
		Jitter: true,
	}

	err = f()

	if err == nil {
		return nil
	}

	for i := 0; i < retry; i++ {
		err = f()

		if err == nil {
			return nil
		}

		b := jitter.Duration()
		time.Sleep(b)
	}

	if err != nil {
		return err
	}

	return nil
}
