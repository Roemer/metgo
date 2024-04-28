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
	siteName                   string
	cacheDir                   string
	lastLocationforecastResult *LocationforecastResult
	lastLocationforecastMeta   cacheMetaData
	debug                      bool
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

// Gets a locationforecast result object.
func (s *MetNoService) Locationforecast(lat float64, lon float64, alt int) (*LocationforecastResult, error) {
	// Check if there are data in-memory which should be reused
	if s.lastLocationforecastResult != nil {
		if !isExpired(s.lastLocationforecastMeta.Expires) {
			if s.debug {
				fmt.Println("Loaded from memory")
			}
			return s.lastLocationforecastResult, nil
		} else {
			if s.debug {
				fmt.Println("Memory cache expired")
			}
		}
	}

	// Check if there are data in the disk-cache that should be reused
	cacheFileName := fmt.Sprintf("metno-%.4f-%.4f-%d.json", lat, lon, alt)
	cacheMetaFileName := fmt.Sprintf("metno-%.4f-%.4f-%d-meta.json", lat, lon, alt)
	cacheData, cacheMeta, err := loadDataFromCache[LocationforecastResult](s.cacheDir, cacheFileName, cacheMetaFileName)
	if err != nil {
		return nil, err
	}
	if cacheData != nil {
		// Found valid cache data
		if s.debug {
			fmt.Println("Loaded from disk cache")
		}

		// Check if the data is expired
		if !isExpired(cacheMeta.Expires) {
			// Make sure that it is stored in memory
			s.lastLocationforecastResult = cacheData
			s.lastLocationforecastMeta = cacheMeta
			return cacheData, nil
		} else {
			if s.debug {
				fmt.Println("Disk cache expired")
			}
		}
	}

	// No data somewhere else, so get the data from the api
	cacheData, cacheMeta, err = s.loadDataFromApi(lat, lon, alt)
	if err != nil {
		return nil, err
	}
	if s.debug {
		fmt.Println("Loaded from api")
	}
	// Update the memory and cache with the new data
	if err := s.updateLocationforecastData(cacheFileName, cacheData, cacheMetaFileName, cacheMeta); err != nil {
		return nil, fmt.Errorf("failed updating the data in the cache: %w", err)
	}

	return cacheData, nil
}

func (s *MetNoService) updateLocationforecastData(cacheFileName string, result *LocationforecastResult, cacheMetaFileName string, metaData cacheMetaData) error {
	// Memory
	s.lastLocationforecastResult = result
	s.lastLocationforecastMeta = metaData

	// Cache
	if s.cacheDir == "" {
		return nil
	}
	// Cache data
	cacheFilePath := filepath.Join(s.cacheDir, cacheFileName)
	cacheString, err := json.MarshalIndent(s.lastLocationforecastResult, "", " ")
	if err != nil {
		return fmt.Errorf("failed converting the data object to a string")
	}
	if err := os.WriteFile(cacheFilePath, cacheString, os.ModePerm); err != nil {
		return fmt.Errorf("failed storing the cache file: %w", err)
	}

	// Cache metadata
	cacheMetaFilePath := filepath.Join(s.cacheDir, cacheMetaFileName)
	cacheMetaString, err := json.MarshalIndent(s.lastLocationforecastMeta, "", " ")
	if err != nil {
		return fmt.Errorf("failed converting the meta data object to a string")
	}
	if err := os.WriteFile(cacheMetaFilePath, cacheMetaString, os.ModePerm); err != nil {
		return fmt.Errorf("failed storing the meta data file: %w", err)
	}
	// All good
	return nil
}

func (s *MetNoService) loadDataFromApi(lat float64, lon float64, alt int) (*LocationforecastResult, cacheMetaData, error) {
	// Create the request
	url := fmt.Sprintf("https://api.met.no/weatherapi/locationforecast/2.0/complete?lat=%.4f&lon=%.4f&altitude=%d", lat, lon, alt)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, cacheMetaData{}, err
	}
	req.Header.Set("User-Agent", s.siteName)

	// TODO:  If-Modified-Since header, using the value of the Last-Modified header

	// Execute the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, cacheMetaData{}, err
	}
	defer resp.Body.Close()

	// TODO: Handle 304 response (Not Modified)

	// Handle the 'Expires' response header
	expiresValue, ok := resp.Header["Expires"]
	if !ok {
		return nil, cacheMetaData{}, fmt.Errorf("failed getting the 'Expires' header")
	}
	expiresDate, err := time.Parse(time.RFC1123, expiresValue[0])
	if err != nil {
		return nil, cacheMetaData{}, fmt.Errorf("failed parsing the expires date: %w", err)
	}
	lastModifiedValue, ok := resp.Header["Last-Modified"]
	if !ok {
		return nil, cacheMetaData{}, fmt.Errorf("failed getting the 'Last-Modified' header")
	}
	lastModifiedDate, err := time.Parse(time.RFC1123, lastModifiedValue[0])
	if err != nil {
		return nil, cacheMetaData{}, fmt.Errorf("failed parsing the last-modified date: %w", err)
	}

	// Read and convert the body
	var dataObject LocationforecastResult
	if err := json.NewDecoder(resp.Body).Decode(&dataObject); err != nil {
		return nil, cacheMetaData{}, fmt.Errorf("error converting the response body to json: %w", err)
	}

	// Return the values
	return &dataObject, cacheMetaData{Expires: expiresDate.Local(), LastModified: lastModifiedDate.Local()}, nil
}

//////////
// Helper methods
//////////

func isExpired(checkDate time.Time) bool {
	return time.Now().After(checkDate)
}

func loadDataFromCache[T interface{}](cacheDir, cacheFileName, cacheMetaFileName string) (*T, cacheMetaData, error) {
	// No cache dir defined, so don't use the cache at all
	if cacheDir == "" {
		return nil, cacheMetaData{}, nil
	}
	// Make sure the cache folder exists
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return nil, cacheMetaData{}, err
	}

	// Try getting the data file
	cacheFilePath := filepath.Join(cacheDir, cacheFileName)
	cacheDataObject, err := readJsonFromFile[T](cacheFilePath, false)
	if err != nil {
		return nil, cacheMetaData{}, err
	} else if cacheDataObject == nil {
		return nil, cacheMetaData{}, nil
	}

	// Try getting the metadata file
	cacheMetaFilePath := filepath.Join(cacheDir, cacheMetaFileName)
	cacheMetaDataObject, err := readJsonFromFile[cacheMetaData](cacheMetaFilePath, false)
	if err != nil {
		return nil, cacheMetaData{}, err
	} else if cacheMetaDataObject == nil {
		return nil, cacheMetaData{}, nil
	}

	// Return the values
	return cacheDataObject, *cacheMetaDataObject, nil
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
