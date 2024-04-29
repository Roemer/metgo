package metgo

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetNoCache(t *testing.T) {
	assert := assert.New(t)

	// Make sure to clear the cache directory
	cacheDirectory := ".metno-cache"
	err := os.RemoveAll(cacheDirectory)
	assert.NoError(err)

	// Initialize the service
	service, err := NewMetNoService("https://github.com/Roemer/metgo", cacheDirectory)
	assert.NoError(err)
	assert.NotNil(service)
	// Enable debug messages
	service.EnableDebug()

	// Get a location
	locationforecastResult, err := service.Locationforecast(59.942787176440405, 10.720651536344942, 100)
	assert.NoError(err)
	assert.NotNil(locationforecastResult)

	// Load the location again (should be from memory cache)
	locationforecastResult, err = service.Locationforecast(59.942787176440405, 10.720651536344942, 100)
	assert.NoError(err)
	assert.NotNil(locationforecastResult)

	// Clear the memory
	service.lastLocationforecastCacheInfo = cacheInfo{}
	service.lastLocationforecastResult = nil

	// Load the location again (should be from disk cache)
	locationforecastResult, err = service.Locationforecast(59.942787176440405, 10.720651536344942, 100)
	assert.NoError(err)
	assert.NotNil(locationforecastResult)

	// Modify the expired header
	service.lastLocationforecastCacheInfo.Expires = time.Now().Add(-30 * time.Minute)
	service.saveCacheInfo(service.lastLocationforecastCacheInfo, "metno-locationforecast-59.9428-10.7207-100-info.json")

	// Load the location again (should re-fetch from api)
	locationforecastResult, err = service.Locationforecast(59.942787176440405, 10.720651536344942, 100)
	assert.NoError(err)
	assert.NotNil(locationforecastResult)
}
