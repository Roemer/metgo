package metgo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type MetNoService struct {
	siteName                      string
	cacheDir                      string
	lastLocationforecastResult    *LocationforecastResult
	lastLocationforecastCacheInfo cacheInfo
	debug                         bool
}

// Method to create a new service to ineract with the met.no api.
func NewMetNoService(siteName string, cacheDirectory string) (*MetNoService, error) {
	if siteName == "" {
		return nil, fmt.Errorf("siteName must be defined")
	}
	return &MetNoService{
		siteName: siteName,
		cacheDir: cacheDirectory,
		debug:    false,
	}, nil
}

func (s *MetNoService) EnableDebug() {
	s.debug = true
}

// Get a locationforecast result.
func (s *MetNoService) Locationforecast(lat float64, lon float64, alt int) (*LocationforecastResult, error) {
	cacheName := fmt.Sprintf("locationforecast-%.4f-%.4f-%d", lat, lon, alt)
	cacheObject, err := prepareDataFromCache[LocationforecastResult](s, s.lastLocationforecastResult, s.lastLocationforecastCacheInfo, cacheName,
		func(diskCacheObject *LocationforecastResult, diskCacheInfoObject cacheInfo) {
			s.lastLocationforecastResult = diskCacheObject
			s.lastLocationforecastCacheInfo = diskCacheInfoObject
		})
	if err != nil {
		return nil, err
	}
	if cacheObject != nil {
		return cacheObject, nil
	}

	// No data somewhere else, so get the data from the api
	url := fmt.Sprintf("https://api.met.no/weatherapi/locationforecast/2.0/complete?lat=%.4f&lon=%.4f&altitude=%d", lat, lon, alt)
	apiCacheObject, apiCacheInfoObject, err := loadDataFromApi(s, url, s.lastLocationforecastResult, s.lastLocationforecastCacheInfo)
	if err != nil {
		return nil, err
	}
	if s.debug {
		fmt.Println("Loaded from api")
	}
	// Update the memory cache
	s.lastLocationforecastResult = apiCacheObject
	s.lastLocationforecastCacheInfo = apiCacheInfoObject
	// Update the disk cache
	if err := s.updateDiskCache(apiCacheObject, apiCacheInfoObject, cacheName); err != nil {
		return nil, fmt.Errorf("failed updating the data in the cache: %w", err)
	}

	return apiCacheObject, nil
}

func (s *MetNoService) updateDiskCache(cacheObject interface{}, cacheInfoObject cacheInfo, cacheName string) error {
	cacheFileName, cacheInfoFileName := getCacheFileNames(cacheName)
	// Cache result in disk cache
	if err := s.saveResult(cacheObject, cacheFileName); err != nil {
		return err
	}
	// Cache info in disk cache
	if err := s.saveCacheInfo(cacheInfoObject, cacheInfoFileName); err != nil {
		return err
	}
	// All good
	return nil
}

func (s *MetNoService) saveResult(cacheObject interface{}, cacheFileName string) error {
	// No disk cache provided, return without saving
	if s.cacheDir == "" {
		return nil
	}
	cacheFilePath := filepath.Join(s.cacheDir, cacheFileName)
	cacheString, err := json.MarshalIndent(cacheObject, "", " ")
	if err != nil {
		return fmt.Errorf("failed converting the data object to a string")
	}
	if err := os.WriteFile(cacheFilePath, cacheString, os.ModePerm); err != nil {
		return fmt.Errorf("failed storing the cache file: %w", err)
	}
	return nil
}

func (s *MetNoService) saveCacheInfo(cacheInfoObject cacheInfo, cacheInfoFileName string) error {
	// No disk cache provided, return without saving
	if s.cacheDir == "" {
		return nil
	}
	cacheInfoFilePath := filepath.Join(s.cacheDir, cacheInfoFileName)
	cacheInfoString, err := json.MarshalIndent(cacheInfoObject, "", " ")
	if err != nil {
		return fmt.Errorf("failed converting the info data object to a string")
	}
	if err := os.WriteFile(cacheInfoFilePath, cacheInfoString, os.ModePerm); err != nil {
		return fmt.Errorf("failed storing the info data file: %w", err)
	}
	return nil
}

////////////////////////////////////////////////////////////
// Helper methods
////////////////////////////////////////////////////////////

func isExpired(checkDate time.Time) bool {
	return time.Now().After(checkDate)
}

// Loads and prepares the data from disk-cache.
func prepareDataFromCache[T interface{}](service *MetNoService, memCacheObject *T, memCacheInfoObject cacheInfo, cacheName string, setMemCacheFunc func(*T, cacheInfo)) (*T, error) {
	// Check the memory cache and if it is not expired, return it directly
	if memCacheObject != nil && !isExpired(memCacheInfoObject.Expires) {
		fmt.Println("Use memory cache as it is not expired")
		return memCacheObject, nil
	}

	// Load data from disk cache
	cacheFileName, cacheInfoFileName := getCacheFileNames(cacheName)
	diskCacheObject, diskCacheInfoObject, err := loadDataFromCache[T](service.cacheDir, cacheFileName, cacheInfoFileName)
	if err != nil {
		return nil, err
	}
	// No disk cache, fast return
	if diskCacheObject == nil {
		fmt.Println("No data in disk cache")
		return nil, nil
	}

	// if the disk cache is newer than the memory cache or the memory cache is empty, update the memory cache with it
	if memCacheObject == nil || diskCacheInfoObject.LastModified.After(memCacheInfoObject.LastModified) {
		fmt.Println("Updated memory cache with disk cache")
		setMemCacheFunc(diskCacheObject, diskCacheInfoObject)
	}

	// Check the disk cache and if it is not expired, return it directly
	if !isExpired(diskCacheInfoObject.Expires) {
		fmt.Println("Use disk cache as it is not expired")
		return diskCacheObject, nil
	}

	// All caches are not set or expired
	fmt.Println("Caches are not set or expired")
	return nil, nil
}

func getCacheFileNames(cacheName string) (string, string) {
	cacheFileName := fmt.Sprintf("metno-%s.json", cacheName)
	cacheInfoFileName := fmt.Sprintf("metno-%s-info.json", cacheName)
	return cacheFileName, cacheInfoFileName
}

func loadDataFromCache[T interface{}](cacheDir, cacheFileName, cacheInfoFileName string) (*T, cacheInfo, error) {
	// No cache dir defined, so don't use the cache at all
	if cacheDir == "" {
		return nil, cacheInfo{}, nil
	}
	// Make sure the cache folder exists
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return nil, cacheInfo{}, err
	}

	// Try getting the data file
	cacheFilePath := filepath.Join(cacheDir, cacheFileName)
	cacheDataObject, err := readJsonFromFile[T](cacheFilePath, false)
	if err != nil {
		return nil, cacheInfo{}, err
	} else if cacheDataObject == nil {
		return nil, cacheInfo{}, nil
	}

	// Try getting the info file
	cacheInfoFilePath := filepath.Join(cacheDir, cacheInfoFileName)
	cacheInfoObject, err := readJsonFromFile[cacheInfo](cacheInfoFilePath, false)
	if err != nil {
		return nil, cacheInfo{}, err
	} else if cacheInfoObject == nil {
		return nil, cacheInfo{}, nil
	}

	// Return the values
	return cacheDataObject, *cacheInfoObject, nil
}

func loadDataFromApi[T interface{}](service *MetNoService, url string, lastCachedData *T, lastCacheInfo cacheInfo) (*T, cacheInfo, error) {
	if service.debug {
		fmt.Printf("Loading data from api url: %s\n", url)
	}
	// Create the request
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, cacheInfo{}, err
	}
	req.Header.Set("User-Agent", service.siteName)
	// Add last modified if we hve the info and cached data
	if !lastCacheInfo.LastModified.IsZero() && lastCachedData != nil {
		gmtTimeLoc := time.FixedZone("GMT", 0)
		ifModifiedDate := lastCacheInfo.LastModified.In(gmtTimeLoc).Format(time.RFC1123)
		req.Header.Set("If-Modified-Since", ifModifiedDate)
		if service.debug {
			fmt.Printf("Adding If-Modified-Since header: %s\n", ifModifiedDate)
		}
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
		if service.debug {
			fmt.Println("data from api not modified")
		}
		// Return the last data but update the cache info
		return lastCachedData, cacheInfo{Expires: expiresDate.Local(), LastModified: lastModifiedDate.Local()}, nil
	}

	// Check if the status code is a success code
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Read and convert the body
		var dataObject T
		if err := json.NewDecoder(resp.Body).Decode(&dataObject); err != nil {
			return nil, cacheInfo{}, fmt.Errorf("error converting the response body to json: %w", err)
		}

		// Return the values
		return &dataObject, cacheInfo{Expires: expiresDate.Local(), LastModified: lastModifiedDate.Local()}, nil
	}

	// Failed status code
	return nil, cacheInfo{}, fmt.Errorf("failed getting new data from the api with code: %d", resp.StatusCode)
}

func readJsonFromFile[T interface{}](filePath string, errorOnNotFound bool) (*T, error) {
	fileDescriptor, err := os.Open(filePath)
	if errors.Is(err, os.ErrNotExist) {
		// File not found
		if errorOnNotFound {
			return nil, fmt.Errorf("file '%s' not found: %w", filePath, err)
		}
		return nil, nil
	} else if err != nil {
		// Error while reading the file
		return nil, fmt.Errorf("error reading the file '%s': %w", filePath, err)
	}
	// Read the file
	defer fileDescriptor.Close()
	var dataObject T
	if err := json.NewDecoder(fileDescriptor).Decode(&dataObject); err != nil {
		return nil, fmt.Errorf("error converting the file '%s' to json: %w", filePath, err)
	}
	return &dataObject, nil
}
