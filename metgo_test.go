package metgo

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetNoCache(t *testing.T) {
	assert := assert.New(t)

	// Prepare
	lat := 59.942787176440405
	lon := 10.720651536344942
	alt := 100

	// Make sure to clear the cache directory
	cacheDirectory := ".metno-cache"
	err := os.RemoveAll(cacheDirectory)
	assert.NoError(err)

	// Initialize the service
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	service, err := NewMetNoService("https://github.com/Roemer/metgo", cacheDirectory, logger)
	assert.NoError(err)
	assert.NotNil(service)

	// Get a location (should be from api)
	locationforecastResult, err := service.Locationforecast(lat, lon, alt)
	assert.NoError(err)
	assert.NotNil(locationforecastResult)

	// Load the location again (should be from memory cache)
	locationforecastResult, err = service.Locationforecast(lat, lon, alt)
	assert.NoError(err)
	assert.NotNil(locationforecastResult)

	// Clear the memory cache
	cacheName := service.buildLocationforecastCacheName(lat, lon, alt)
	service.locationForecastCaches[0].ClearCache(cacheName)

	// Load the location again (should be from disk cache)
	locationforecastResult, err = service.Locationforecast(lat, lon, alt)
	assert.NoError(err)
	assert.NotNil(locationforecastResult)

	// Modify the expired header
	obj, info, _ := service.locationForecastCaches[0].GetCache(cacheName)
	info.Expires = info.Expires.Add(-24 * time.Hour)
	service.locationForecastCaches[0].SetCache(cacheName, obj, info)
	service.locationForecastCaches[1].SetCache(cacheName, obj, info)

	// Load the location again (should re-fetch from api but with a 304 - not modified)
	locationforecastResult, err = service.Locationforecast(lat, lon, alt)
	assert.NoError(err)
	assert.NotNil(locationforecastResult)
}
