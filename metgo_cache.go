package metgo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Cache[T any] interface {
	GetCache(cacheName string) (*T, cacheInfo, error)
	SetCache(cacheName string, cacheObject *T, cacheInfoObject cacheInfo) error
	ClearCache(cacheName string) error
}

////////////////////////////////////////////////////////////
// Memory Cache
////////////////////////////////////////////////////////////

type MemoryCache[T any] struct {
	CacheObject     map[string]*T
	CacheInfoObject map[string]cacheInfo
}

func (m *MemoryCache[T]) GetCache(cacheName string) (*T, cacheInfo, error) {
	m.makeSureMapIsInitialized()
	return m.CacheObject[cacheName], m.CacheInfoObject[cacheName], nil
}

func (m *MemoryCache[T]) SetCache(cacheName string, cacheObject *T, cacheInfoObject cacheInfo) error {
	m.makeSureMapIsInitialized()
	m.CacheObject[cacheName] = cacheObject
	m.CacheInfoObject[cacheName] = cacheInfoObject
	return nil
}

func (m *MemoryCache[T]) ClearCache(cacheName string) error {
	m.makeSureMapIsInitialized()
	delete(m.CacheObject, cacheName)
	delete(m.CacheInfoObject, cacheName)
	return nil
}

func (m *MemoryCache[T]) makeSureMapIsInitialized() {
	if m.CacheObject == nil {
		m.CacheObject = map[string]*T{}
	}
	if m.CacheInfoObject == nil {
		m.CacheInfoObject = map[string]cacheInfo{}
	}
}

////////////////////////////////////////////////////////////
// Disk Cache
////////////////////////////////////////////////////////////

type DiskCache[T any] struct {
	CacheDirectory string
}

func (m *DiskCache[T]) GetCache(cacheName string) (*T, cacheInfo, error) {
	// No directory provided, return without saving
	if m.CacheDirectory == "" {
		return nil, cacheInfo{}, nil
	}

	// Get the file names
	cacheFileName, cacheInfoFileName := m.getCacheFileNames(cacheName)

	// Try getting the data file
	cacheFilePath := filepath.Join(m.CacheDirectory, cacheFileName)
	cacheDataObject, err := readJsonFromFile[T](cacheFilePath, false)
	if err != nil {
		return nil, cacheInfo{}, err
	} else if cacheDataObject == nil {
		return nil, cacheInfo{}, nil
	}

	// Try getting the info file
	cacheInfoFilePath := filepath.Join(m.CacheDirectory, cacheInfoFileName)
	cacheInfoObject, err := readJsonFromFile[cacheInfo](cacheInfoFilePath, false)
	if err != nil {
		return nil, cacheInfo{}, err
	} else if cacheInfoObject == nil {
		return nil, cacheInfo{}, nil
	}

	// Return the values
	return cacheDataObject, *cacheInfoObject, nil
}

func (m *DiskCache[T]) SetCache(cacheName string, cacheObject *T, cacheInfoObject cacheInfo) error {
	// No directory provided, return without saving
	if m.CacheDirectory == "" {
		return nil
	}

	// Make sure the cache folder exists
	if err := os.MkdirAll(m.CacheDirectory, os.ModePerm); err != nil {
		return err
	}

	// Get the file names
	cacheFileName, cacheInfoFileName := m.getCacheFileNames(cacheName)

	// CacheObject
	cacheFilePath := filepath.Join(m.CacheDirectory, cacheFileName)
	cacheString, err := json.MarshalIndent(cacheObject, "", " ")
	if err != nil {
		return fmt.Errorf("failed converting the data object to a string")
	}
	if err := os.WriteFile(cacheFilePath, cacheString, os.ModePerm); err != nil {
		return fmt.Errorf("failed storing the cache file: %w", err)
	}

	// CacheInfoObject
	cacheInfoFilePath := filepath.Join(m.CacheDirectory, cacheInfoFileName)
	cacheInfoString, err := json.MarshalIndent(cacheInfoObject, "", " ")
	if err != nil {
		return fmt.Errorf("failed converting the info object to a string")
	}
	if err := os.WriteFile(cacheInfoFilePath, cacheInfoString, os.ModePerm); err != nil {
		return fmt.Errorf("failed storing the info file: %w", err)
	}

	return nil
}

func (m *DiskCache[T]) ClearCache(cacheName string) error {
	cacheFileName, cacheInfoFileName := m.getCacheFileNames(cacheName)
	cacheFilePath := filepath.Join(m.CacheDirectory, cacheFileName)
	cacheInfoFilePath := filepath.Join(m.CacheDirectory, cacheInfoFileName)
	os.Remove(cacheFilePath)
	os.Remove(cacheInfoFilePath)
	return nil
}

func (m *DiskCache[T]) getCacheFileNames(cacheName string) (string, string) {
	cacheFileName := fmt.Sprintf("metno-%s.json", cacheName)
	cacheInfoFileName := fmt.Sprintf("metno-%s-info.json", cacheName)
	return cacheFileName, cacheInfoFileName
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
