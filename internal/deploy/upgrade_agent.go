package deploy

import "context"

type handoffContextKey struct{}

func withHandoff(ctx context.Context, requests <-chan struct{}) context.Context {
	return context.WithValue(ctx, handoffContextKey{}, requests)
}

func handoffRequested(ctx context.Context) bool {
	requests, _ := ctx.Value(handoffContextKey{}).(<-chan struct{})
	select {
	case <-requests:
		return true
	default:
		return false
	}
}
