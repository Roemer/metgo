package metgo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type MetNoService struct {
	siteName               string
	cacheDir               string
	logger                 *slog.Logger
	locationForecastCaches []Cache[LocationforecastResult]
}

// Method to create a new service to interact with the met.no api.
func NewMetNoService(siteName string, cacheDirectory string, logger *slog.Logger) (*MetNoService, error) {
	if siteName == "" {
		return nil, fmt.Errorf("siteName must be defined")
	}
	if logger == nil {
		logger = slog.New(discardHandler{})
	}
	service := &MetNoService{
		siteName: siteName,
		cacheDir: cacheDirectory,
		logger:   logger,
		// Caches should be ordered from most to least volatile (or performant)
		locationForecastCaches: []Cache[LocationforecastResult]{
			&MemoryCache[LocationforecastResult]{},
			&DiskCache[LocationforecastResult]{CacheDirectory: cacheDirectory},
		},
	}
	return service, nil
}

// Get a locationforecast result.
func (s *MetNoService) Locationforecast(lat float64, lon float64, alt int) (*LocationforecastResult, error) {
	// Prepare the cache name
	cacheName := s.buildLocationforecastCacheName(lat, lon, alt)

	// Try get the data from one of the caches
	cacheObject, cacheInfo, err := getDataFromCaches(s, s.locationForecastCaches, cacheName)
	if err != nil {
		return nil, err
	}
	// If we have a cache object which is not expired, return it
	if cacheObject != nil && !isExpired(cacheInfo.Expires) {
		s.logger.Debug("Found valid data in cache")
		return cacheObject, nil
	}

	// No data somewhere else, so get the data from the api
	url := fmt.Sprintf("https://api.met.no/weatherapi/locationforecast/2.0/complete?lat=%.4f&lon=%.4f&altitude=%d", lat, lon, alt)
	apiCacheObject, apiCacheInfoObject, err := loadDataFromApi(s, url, cacheObject, cacheInfo)
	if err != nil {
		return nil, err
	}
	s.logger.Debug("Loaded from api")

	// Update the caches
	for _, cache := range s.locationForecastCaches {
		if err := cache.SetCache(cacheName, apiCacheObject, apiCacheInfoObject); err != nil {
			return nil, err
		}
	}

	// Return the objec
	return apiCacheObject, nil
}

func (s *MetNoService) buildLocationforecastCacheName(lat float64, lon float64, alt int) string {
	return fmt.Sprintf("locationforecast-%.4f-%.4f-%d", lat, lon, alt)
}

////////////////////////////////////////////////////////////
// Helper methods
////////////////////////////////////////////////////////////

func isExpired(checkDate time.Time) bool {
	return time.Now().After(checkDate)
}

func getDataFromCaches[T any](service *MetNoService, caches []Cache[T], cacheName string) (*T, cacheInfo, error) {
	// Prepare variables to store the newest result from any of the caches
	var newestObj *T
	var newestInfo cacheInfo
	var newestIndex int
	// Prepare a map with the last modified date for each processed cache
	cacheLastModified := map[int]time.Time{}
	// Loop thru the caches
	for i, cache := range caches {
		// Try get the objects from this cache
		obj, info, err := cache.GetCache(cacheName)
		if err != nil {
			return nil, cacheInfo{}, err
		}
		if obj == nil {
			// Object not cached, continue with next cache
			service.logger.Debug(fmt.Sprintf("No data in cache %d", i))
			continue
		}

		// Store the data if it is the newest of all caches (or the first that has data)
		if newestObj == nil || newestInfo.LastModified.Before(info.LastModified) {
			newestObj = obj
			newestInfo = info
			newestIndex = i
		}

		// If the object is not expired, stop processing caches
		if !isExpired(info.Expires) {
			service.logger.Debug(fmt.Sprintf("Data in cache %d is not expired, using it", i))
			break
		}
		service.logger.Debug(fmt.Sprintf("Data in cache %d is expired, trying next cache", i))

		// Store the last modified date of this cache
		cacheLastModified[i] = info.LastModified
	}

	// No data in all caches found
	if newestObj == nil {
		service.logger.Debug("No data in all caches")
		return nil, cacheInfo{}, nil
	}

	// If the higher-rated caches had no or an older result, update it
	for i := 0; i < newestIndex; i++ {
		prevCacheModified, ok := cacheLastModified[i]
		if !ok || prevCacheModified.Before(newestInfo.LastModified) {
			service.logger.Debug(fmt.Sprintf("Update data in cache %d from cache %d", i, newestIndex))
			if err := caches[i].SetCache(cacheName, newestObj, newestInfo); err != nil {
				return nil, cacheInfo{}, nil
			}
		}
	}

	// Return the data
	return newestObj, newestInfo, nil
}

func loadDataFromApi[T interface{}](service *MetNoService, url string, lastCachedData *T, lastCacheInfo cacheInfo) (*T, cacheInfo, error) {
	service.logger.Debug(fmt.Sprintf("Loading data from api url: %s", url))
	// Create the request
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, cacheInfo{}, err
	}
	req.Header.Set("User-Agent", service.siteName)
	// Add last modified if we have the info and cached data
	if !lastCacheInfo.LastModified.IsZero() && lastCachedData != nil {
		gmtTimeLoc := time.FixedZone("GMT", 0)
		ifModifiedDate := lastCacheInfo.LastModified.In(gmtTimeLoc).Format(time.RFC1123)
		req.Header.Set("If-Modified-Since", ifModifiedDate)
		service.logger.Debug(fmt.Sprintf("Adding If-Modified-Since header: %s", ifModifiedDate))
	}

	// Execute the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, cacheInfo{}, err
	}
	defer resp.Body.Close()

	// Get the response headers regarding times
	expiresValue, ok := resp.Header["Expires"]
	if !ok {
		return nil, cacheInfo{}, fmt.Errorf("failed getting the 'Expires' header")
	}
	expiresDate, err := time.Parse(time.RFC1123, expiresValue[0])
	if err != nil {
		return nil, cacheInfo{}, fmt.Errorf("failed parsing the expires date: %w", err)
	}
	lastModifiedValue, ok := resp.Header["Last-Modified"]
	if !ok {
		return nil, cacheInfo{}, fmt.Errorf("failed getting the 'Last-Modified' header")
	}
	lastModifiedDate, err := time.Parse(time.RFC1123, lastModifiedValue[0])
	if err != nil {
		return nil, cacheInfo{}, fmt.Errorf("failed parsing the last-modified date: %w", err)
	}

	// Check if the response was 304 - Not Modified
	if resp.StatusCode == 304 {
		service.logger.Debug("Data from api not modified")
		// Return the last data but update the cache info
		return lastCachedData, cacheInfo{Expires: expiresDate, LastModified: lastModifiedDate}, nil
	}

	// Check if the status code is a success code
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Read and convert the body
		var dataObject T
		if err := json.NewDecoder(resp.Body).Decode(&dataObject); err != nil {
			return nil, cacheInfo{}, fmt.Errorf("error converting the response body to json: %w", err)
		}

		// Return the values
		return &dataObject, cacheInfo{Expires: expiresDate, LastModified: lastModifiedDate}, nil
	}

	// Failed status code
	return nil, cacheInfo{}, fmt.Errorf("failed getting new data from the api with code: %d", resp.StatusCode)
}
