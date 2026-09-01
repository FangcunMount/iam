package jwks

func buildDefaultChain(b *managerBuilder) KeyFetcher {
	var tail KeyFetcher
	if len(b.seedData) > 0 {
		seedFetcher, _ := NewSeedFetcher(b.seedData)
		if seedFetcher.keySet != nil {
			tail = seedFetcher
		}
	}

	if b.enableGRPC && b.authClient != nil {
		tail = NewGRPCFetcher(b.authClient, WithGRPCNext(tail))
	} else if b.config.GRPCEndpoint != "" {
		tail = NewGRPCEndpointFetcher(b.config.GRPCEndpoint, WithGRPCEndpointNext(tail))
	}

	httpOpts := []HTTPFetcherOption{
		WithHTTPTimeout(b.config.RequestTimeout),
		WithHTTPNext(tail),
	}
	if b.config.HTTPClient != nil {
		httpOpts = append(httpOpts, WithHTTPClient(b.config.HTTPClient))
	}
	if len(b.config.CustomHeaders) > 0 {
		httpOpts = append(httpOpts, WithHTTPHeaders(b.config.CustomHeaders))
	}
	tail = NewHTTPFetcher(b.config.URL, httpOpts...)

	if b.enableCircuitBreaker {
		tail = NewCircuitBreakerFetcher(tail, b.cbConfig)
	}

	if b.enableCache {
		cacheOptions := []CacheFetcherOption{
			WithCacheNext(tail),
			WithCacheFallbackOnError(b.config.FallbackOnError),
			WithCacheMaxStale(b.config.CacheTTL),
		}
		refreshAfter := b.config.RefreshInterval
		if b.config.CacheTTL > 0 && (refreshAfter <= 0 || refreshAfter > b.config.CacheTTL) {
			refreshAfter = b.config.CacheTTL
		}
		if refreshAfter > 0 {
			cacheOptions = append(cacheOptions, WithCacheTTL(refreshAfter))
		}
		return NewCacheFetcher(cacheOptions...)
	}

	return tail
}
