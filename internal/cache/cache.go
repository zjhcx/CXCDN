package cache

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"google.golang.org/protobuf/proto"
)

var C *cache.Cache

// DefaultExpiration re-exports cache.DefaultExpiration for use by other packages
var DefaultExpiration = cache.DefaultExpiration

var (
	cacheFile string
	stopCh    chan struct{}
	once      sync.Once
)

// ValueMarshaler converts an in-memory cache value to a protobuf CacheEntry value.
// Packages that store custom types in the cache should register a marshaler.
type ValueMarshaler func(key string, value interface{}, expiration int64) (*CacheEntry, error)

// ValueUnmarshaler converts a protobuf CacheEntry back to an in-memory value.
type ValueUnmarshaler func(entry *CacheEntry) (interface{}, error)

var (
	marshalers   []ValueMarshaler
	unmarshalers []ValueUnmarshaler
)

// RegisterMarshaler adds a value marshaler for custom types.
// Should be called in init() of the package that owns the type.
func RegisterMarshaler(m ValueMarshaler) {
	marshalers = append(marshalers, m)
}

// RegisterUnmarshaler adds a value unmarshaler for custom types.
func RegisterUnmarshaler(u ValueUnmarshaler) {
	unmarshalers = append(unmarshalers, u)
}

// Init initializes the in-memory cache and optionally loads from disk.
// If cacheFile is non-empty, it loads existing cache from that file and
// starts a background goroutine to periodically save the cache back.
func Init(cacheFilePath string) {
	C = cache.New(30*time.Minute, 10*time.Minute)

	if cacheFilePath == "" {
		return
	}

	cacheFile = cacheFilePath

	// Load from disk
	loadFromDisk()

	// Start periodic save
	once.Do(func() {
		stopCh = make(chan struct{})
		go periodicSave()
	})
}

// loadFromDisk reads the protobuf cache file and restores entries.
func loadFromDisk() {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[cache] No disk cache file found at %s, starting fresh", cacheFile)
		} else {
			log.Printf("[cache] Failed to read disk cache: %v", err)
		}
		return
	}

	var cf CacheFile
	if err := proto.Unmarshal(data, &cf); err != nil {
		log.Printf("[cache] Failed to parse disk cache: %v", err)
		return
	}

	count := 0
	now := time.Now().UnixNano()
	for _, entry := range cf.Entries {
		// Skip expired entries
		if entry.ExpirationNs > 0 && entry.ExpirationNs < now {
			continue
		}
		// Calculate remaining TTL
		var ttl time.Duration
		if entry.ExpirationNs > 0 {
			ttl = time.Duration(entry.ExpirationNs-now) * time.Nanosecond
			if ttl <= 0 {
				continue
			}
		} else {
			ttl = cache.DefaultExpiration
		}

		val, err := protoEntryToValue(entry)
		if err != nil {
			continue
		}
		C.Set(entry.Key, val, ttl)
		count++
	}
	log.Printf("[cache] Loaded %d entries from %s", count, cacheFile)
}

// SaveToDisk persists the current cache to disk using protobuf encoding.
func SaveToDisk() {
	if cacheFile == "" {
		return
	}

	items := C.Items()
	cf := &CacheFile{
		Entries: make([]*CacheEntry, 0, len(items)),
	}

	now := time.Now().UnixNano()
	for k, v := range items {
		// Skip expired entries
		if v.Expiration > 0 && v.Expiration < now {
			continue
		}

		entry, err := valueToProtoEntry(k, v.Object, v.Expiration)
		if err != nil {
			continue // skip unsupported types
		}
		cf.Entries = append(cf.Entries, entry)
	}

	data, err := proto.Marshal(cf)
	if err != nil {
		log.Printf("[cache] Failed to marshal cache for disk: %v", err)
		return
	}

	// Write atomically via temp file
	tmp := cacheFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		log.Printf("[cache] Failed to write disk cache: %v", err)
		return
	}
	if err := os.Rename(tmp, cacheFile); err != nil {
		log.Printf("[cache] Failed to rename disk cache: %v", err)
		return
	}

	log.Printf("[cache] Saved %d entries to %s", len(cf.Entries), cacheFile)
}

// Stop signals the periodic save goroutine to stop and performs a final save.
func Stop() {
	if stopCh != nil {
		close(stopCh)
	}
	SaveToDisk()
}

// periodicSave writes the cache to disk every 5 minutes.
func periodicSave() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			SaveToDisk()
		case <-stopCh:
			return
		}
	}
}

// --- value <-> protobuf conversion ---

func valueToProtoEntry(key string, value interface{}, expiration int64) (*CacheEntry, error) {
	entry := &CacheEntry{
		Key:          key,
		ExpirationNs: expiration,
	}

	// Handle built-in types first
	switch v := value.(type) {
	case string:
		entry.Value = &CacheEntry_StringValue{StringValue: v}
		return entry, nil
	case bool:
		entry.Value = &CacheEntry_BoolValue{BoolValue: v}
		return entry, nil
	case []string:
		entry.Value = &CacheEntry_StringList{StringList: &StringList{Items: v}}
		return entry, nil
	case [2]string:
		entry.Value = &CacheEntry_FileContent{FileContent: &FileContent{
			Content:     v[0],
			ContentType: v[1],
		}}
		return entry, nil
	case []byte:
		entry.Value = &CacheEntry_BytesValue{BytesValue: v}
		return entry, nil
	}

	// Try registered marshalers for custom types
	for _, m := range marshalers {
		if e, err := m(key, value, expiration); err == nil {
			return e, nil
		}
	}

	return nil, fmt.Errorf("unsupported cache value type: %T", value)
}

func protoEntryToValue(entry *CacheEntry) (interface{}, error) {
	switch v := entry.Value.(type) {
	case *CacheEntry_StringValue:
		return v.StringValue, nil
	case *CacheEntry_BoolValue:
		return v.BoolValue, nil
	case *CacheEntry_StringList:
		return v.StringList.Items, nil
	case *CacheEntry_FileContent:
		return [2]string{v.FileContent.Content, v.FileContent.ContentType}, nil
	case *CacheEntry_BytesValue:
		return v.BytesValue, nil
	}

	// Try registered unmarshalers for custom types
	for _, u := range unmarshalers {
		if val, err := u(entry); err == nil {
			return val, nil
		}
	}

	return nil, fmt.Errorf("unknown cache entry value type")
}
