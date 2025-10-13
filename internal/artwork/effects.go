package artwork

import (
	"fmt"
	"sync"
)

var (
	effectRegistry   = make(map[string]Effect)
	effectRegistryMu sync.RWMutex
)

// RegisterEffect registers an effect in the global registry
func RegisterEffect(effect Effect) {
	effectRegistryMu.Lock()
	defer effectRegistryMu.Unlock()
	effectRegistry[effect.Name()] = effect
}

// GetEffect retrieves an effect by name from the registry
func GetEffect(name string) (Effect, error) {
	effectRegistryMu.RLock()
	defer effectRegistryMu.RUnlock()

	effect, ok := effectRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unknown effect: %s", name)
	}
	return effect, nil
}

// GetRegisteredEffects returns a list of all registered effect names
func GetRegisteredEffects() []string {
	effectRegistryMu.RLock()
	defer effectRegistryMu.RUnlock()

	names := make([]string, 0, len(effectRegistry))
	for name := range effectRegistry {
		names = append(names, name)
	}
	return names
}

// getIntParam extracts an integer parameter with a default value
func getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return defaultValue
}

// getFloat64Param extracts a float64 parameter with a default value
func getFloat64Param(params map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
	}
	return defaultValue
}

// getStringParam extracts a string parameter with a default value
func getStringParam(params map[string]interface{}, key string, defaultValue string) string {
	if val, ok := params[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultValue
}
