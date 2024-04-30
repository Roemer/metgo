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
	// Check if there are data in-memory which should be reused
	if cacheObject := validateMemoryCache(s, s.lastLocationforecastResult, s.lastLocationforecastCacheInfo); cacheObject != nil {
		// The data object is valid, return it
		return cacheObject, nil
	}

	// Check if there are data in the disk cache that should be reused
	cacheFileName := fmt.Sprintf("metno-locationforecast-%.4f-%.4f-%d.json", lat, lon, alt)
	cacheInfoFileName := fmt.Sprintf("metno-locationforecast-%.4f-%.4f-%d-info.json", lat, lon, alt)
	cacheObject, cacheInfoObject, err := validateDiskCache[LocationforecastResult](s, cacheFileName, cacheInfoFileName)
	if err != nil {
		return nil, err
	}
	if cacheObject != nil {
		// Valid cache data, make sure the disk cache data is stored in memory
		s.lastLocationforecastResult = cacheObject
		s.lastLocationforecastCacheInfo = cacheInfoObject
		// Return the object
		return cacheObject, nil
	}

	// No data somewhere else, so get the data from the api
	url := fmt.Sprintf("https://api.met.no/weatherapi/locationforecast/2.0/complete?lat=%.4f&lon=%.4f&altitude=%d", lat, lon, alt)
	cacheObject, cacheInfoObject, err = loadDataFromApi(s, url, s.lastLocationforecastResult, s.lastLocationforecastCacheInfo)
	if err != nil {
		return nil, err
	}
	if s.debug {
		fmt.Println("Loaded from api")
	}
	// Update the memory cache
	s.lastLocationforecastResult = cacheObject
	s.lastLocationforecastCacheInfo = cacheInfoObject
	// Update the disk cache
	if err := s.updateDiskCache(cacheObject, cacheFileName, cacheInfoObject, cacheInfoFileName); err != nil {
		return nil, fmt.Errorf("failed updating the data in the cache: %w", err)
	}

	return cacheObject, nil
}

func (s *MetNoService) updateDiskCache(cacheObject interface{}, cacheFileName string, cacheInfoObject cacheInfo, cacheInfoFileName string) error {
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

func validateMemoryCache[T interface{}](service *MetNoService, cacheObject *T, cacheInfoObject cacheInfo) *T {
	if cacheObject == nil {
		if service.debug {
			fmt.Println("No data in memory cache")
		}
		return nil
	}
	if isExpired(cacheInfoObject.Expires) {
		if service.debug {
			fmt.Println("Memory cache expired")
		}
		return nil
	}
	if service.debug {
		fmt.Println("Loaded from memory")
	}
	return cacheObject
}

func validateDiskCache[T interface{}](service *MetNoService, cacheFileName string, cacheInfoFileName string) (*T, cacheInfo, error) {
	cacheObject, cacheInfoObject, err := loadDataFromCache[T](service.cacheDir, cacheFileName, cacheInfoFileName)
	if err != nil {
		return nil, cacheInfo{}, err
	}
	if cacheObject == nil {
		// No cached data
		if service.debug {
			fmt.Println("No data in disk cache")
		}
		return nil, cacheInfo{}, nil
	}
	if service.debug {
		fmt.Println("Found data in disk cache")
	}
	// Check if the data from the disk cache expired
	if isExpired(cacheInfoObject.Expires) {
		if service.debug {
			fmt.Println("Disk cache expired")
		}
		return nil, cacheInfo{}, nil
	}
	// Valid disk data
	return cacheObject, cacheInfoObject, nil
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
