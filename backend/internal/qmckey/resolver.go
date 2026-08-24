package qmckey

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrUnavailable    = errors.New("qq music key service unavailable")
	ErrNotLoggedIn    = errors.New("qq music session not found")
	ErrSessionExpired = errors.New("qq music session expired")
	ErrEntitlement    = errors.New("qq music account cannot access resource")
	ErrNetwork        = errors.New("qq music key request failed")
	ErrProtocol       = errors.New("qq music key response invalid")
)

type Resource struct {
	SongID   uint32
	MediaMid string
	Filename string
}

type Resolver interface {
	Resolve(context.Context, Resource) (string, error)
}

type BatchResult struct {
	Resource Resource
	EKey     string
	Err      error
}

type BatchResolver interface {
	Resolver
	ResolveBatch(context.Context, []Resource) []BatchResult
}

type credentials struct {
	uin        string
	authTokens []string
}

type credentialSource interface {
	Load(context.Context) (credentials, error)
}

type keyFetcher interface {
	Fetch(context.Context, string, string, Resource) (string, error)
}

type service struct {
	source  credentialSource
	fetcher keyFetcher
}

func NewDefaultResolver() BatchResolver {
	return &service{
		source:  newLocalCredentialSource(),
		fetcher: newEVKeyClient(nil),
	}
}

func (s *service) Resolve(ctx context.Context, resource Resource) (string, error) {
	results := s.ResolveBatch(ctx, []Resource{resource})
	if len(results) != 1 {
		return "", ErrProtocol
	}
	return results[0].EKey, results[0].Err
}

func (s *service) ResolveBatch(ctx context.Context, resources []Resource) []BatchResult {
	batchCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	ctx = batchCtx

	results := make([]BatchResult, len(resources))
	validResources := 0
	for i, resource := range resources {
		results[i].Resource = resource
		if err := validateResource(resource); err != nil {
			results[i].Err = err
		} else {
			validResources++
		}
	}
	if len(resources) == 0 || validResources == 0 {
		return results
	}

	creds, err := s.source.Load(ctx)
	if err != nil {
		for i := range results {
			if results[i].Err == nil {
				results[i].Err = err
			}
		}
		return results
	}
	if len(creds.authTokens) == 0 {
		for i := range results {
			if results[i].Err == nil {
				results[i].Err = ErrNotLoggedIn
			}
		}
		return results
	}

	const maxConcurrentFetches = 4
	jobs := make(chan int, len(results))
	workerCount := maxConcurrentFetches
	if len(results) < workerCount {
		workerCount = len(results)
	}
	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				results[index].EKey, results[index].Err = s.fetchWithCredentials(ctx, creds, results[index].Resource)
			}
		}()
	}
	for index := range results {
		if results[index].Err == nil {
			jobs <- index
		}
	}
	close(jobs)
	workers.Wait()
	return results
}

func (s *service) fetchWithCredentials(ctx context.Context, creds credentials, resource Resource) (string, error) {
	var lastErr error
	for _, token := range creds.authTokens {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		ekey, err := s.fetcher.Fetch(ctx, creds.uin, token, resource)
		if err == nil {
			return ekey, nil
		}
		lastErr = err
		if errors.Is(err, ErrEntitlement) || errors.Is(err, ErrProtocol) || errors.Is(err, ErrNetwork) {
			return "", err
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", ErrNotLoggedIn
}
