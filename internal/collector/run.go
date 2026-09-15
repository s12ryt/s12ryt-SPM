package collector

import (
	"context"
	"net/http"
	"time"
)

func Run(ctx context.Context, client *http.Client, endpoint, id, token string, report func(error)) error {
	interval := 3 * time.Second
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		sampleCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		sample, err := Collect(sampleCtx)
		cancel()
		if err == nil {
			var next time.Duration
			next, err = Upload(ctx, client, endpoint, id, token, sample)
			if err == nil {
				interval = next
			}
		}
		if err != nil && ctx.Err() == nil {
			report(err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
