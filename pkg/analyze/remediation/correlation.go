package remediation

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CorrelationEngine analyzes relationships between analysis results and remediation steps
type CorrelationEngine struct {
	algorithms []CorrelationAlgorithm
	thresholds CorrelationThresholds
	cache      *CorrelationCache
}

// CorrelationAlgorithm defines an algorithm for finding correlations
type CorrelationAlgorithm interface {
	Name() string
	FindCorrelations(results []AnalysisResult, steps []RemediationStep) []Correlation
	GetStrength(result1, result2 AnalysisResult) float64
}

// CorrelationThresholds defines thresholds for correlation detection
type CorrelationThresholds struct {
	MinStrength   float64       `json:"min_strength"`   // Minimum correlation strength to report
	MinConfidence float64       `json:"min_confidence"` // Minimum confidence to report
	MaxResults    int           `json:"max_results"`    // Maximum correlations to return
	TimeWindow    time.Duration `json:"time_window"`    // Time window for temporal correlations
}

// CorrelationCache caches correlation results to improve performance
type CorrelationCache struct {
	entries map[string]CacheEntry
	maxSize int
	ttl     time.Duration
}

// CacheEntry represents a cached correlation result
type CacheEntry struct {
	Correlations []Correlation `json:"correlations"`
	Timestamp    time.Time     `json:"timestamp"`
	Hash         string        `json:"hash"`
}

// NewCorrelationEngine creates a new correlation engine
func NewCorrelationEngine() *CorrelationEngine {
	return &CorrelationEngine{
		algorithms: []CorrelationAlgorithm{
			&CausalCorrelationAlgorithm{},
			&TemporalCorrelationAlgorithm{},
			&SpatialCorrelationAlgorithm{},
			&FunctionalCorrelationAlgorithm{},
			&ResourceCorrelationAlgorithm{},
		},
		thresholds: CorrelationThresholds{
			MinStrength:   0.3,
			MinConfidence: 0.5,
			MaxResults:    20,
			TimeWindow:    24 * time.Hour,
		},
		cache: &CorrelationCache{
			entries: make(map[string]CacheEntry),
			maxSize: 1000,
			ttl:     30 * time.Minute,
		},
	}
}

// FindCorrelations finds correlations between analysis results and remediation steps
func (ce *CorrelationEngine) FindCorrelations(results []AnalysisResult, steps []RemediationStep) []Correlation {
	// Check cache first
	cacheKey := ce.generateCacheKey(results, steps)
	if cached, found := ce.cache.Get(cacheKey); found {
		return cached.Correlations
	}

	var allCorrelations []Correlation

	// Run each correlation algorithm
	for _, algorithm := range ce.algorithms {
		correlations := algorithm.FindCorrelations(results, steps)
		allCorrelations = append(allCorrelations, correlations...)
	}

	// Filter and rank correlations
	filteredCorrelations := ce.filterCorrelations(allCorrelations)
	rankedCorrelations := ce.rankCorrelations(filteredCorrelations)

	// Limit results
	if len(rankedCorrelations) > ce.thresholds.MaxResults {
		rankedCorrelations = rankedCorrelations[:ce.thresholds.MaxResults]
	}

	// Cache results
	ce.cache.Put(cacheKey, CacheEntry{
		Correlations: rankedCorrelations,
		Timestamp:    time.Now(),
		Hash:         cacheKey,
	})

	return rankedCorrelations
}

// CausalCorrelationAlgorithm finds causal relationships between results
type CausalCorrelationAlgorithm struct{}

func (a *CausalCorrelationAlgorithm) Name() string {
	return "causal"
}

func (a *CausalCorrelationAlgorithm) FindCorrelations(results []AnalysisResult, steps []RemediationStep) []Correlation {
	var correlations []Correlation

	// Look for causal patterns (one issue causes another)
	for i, result1 := range results {
		for j, result2 := range results {
			if i >= j {
				continue
			}

			strength := a.GetStrength(result1, result2)
			if strength < 0.3 {
				continue
			}

			// Check for causal keywords
			causalIndicators := []string{"caused by", "due to", "because of", "resulted from"}
			description1 := strings.ToLower(result1.Description)
			description2 := strings.ToLower(result2.Description)

			foundCausal := false
			for _, indicator := range causalIndicators {
				if strings.Contains(description1, indicator) || strings.Contains(description2, indicator) {
					foundCausal = true
					break
				}
			}

			if foundCausal || strength > 0.7 {
				correlations = append(correlations, Correlation{
					ID:          uuid.New().String(),
					Type:        CorrelationCausal,
					Description: fmt.Sprintf("Potential causal relationship between %s and %s", result1.Title, result2.Title),
					Strength:    strength,
					Confidence:  a.calculateCausalConfidence(result1, result2, strength),
					Results:     []string{result1.ID, result2.ID},
					Evidence: []string{
						fmt.Sprintf("Category correlation: %s -> %s", result1.Category, result2.Category),
						fmt.Sprintf("Strength: %.2f", strength),
					},
					Impact: a.calculateCausalImpact(result1, result2),
				})
			}
		}
	}

	return correlations
}

func (a *CausalCorrelationAlgorithm) GetStrength(result1, result2 AnalysisResult) float64 {
	// Calculate correlation strength based on various factors
	categoryMatch := a.calculateCategoryCorrelation(result1.Category, result2.Category)
	keywordMatch := a.calculateKeywordCorrelation(result1.Description, result2.Description)
	severityCorrelation := a.calculateSeverityCorrelation(result1.Severity, result2.Severity)

	// Weighted combination
	return (categoryMatch*0.4 + keywordMatch*0.4 + severityCorrelation*0.2)
}

func (a *CausalCorrelationAlgorithm) calculateCategoryCorrelation(cat1, cat2 string) float64 {
	if cat1 == cat2 {
		return 1.0
	}

	// Define category relationships
	categoryRelations := map[string][]string{
		"resource":      {"storage", "network", "application"},
		"storage":       {"resource", "application"},
		"network":       {"resource", "security", "application"},
		"security":      {"network", "application"},
		"configuration": {"resource", "storage", "network", "security", "application"},
	}

	if relations, exists := categoryRelations[strings.ToLower(cat1)]; exists {
		for _, related := range relations {
			if strings.ToLower(cat2) == related {
				return 0.6
			}
		}
	}

	return 0.1
}

func (a *CausalCorrelationAlgorithm) calculateKeywordCorrelation(desc1, desc2 string) float64 {
	words1 := strings.Fields(strings.ToLower(desc1))
	words2 := strings.Fields(strings.ToLower(desc2))

	commonWords := 0
	totalWords := len(words1) + len(words2)

	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 && len(word1) > 3 { // Ignore short words
				commonWords++
			}
		}
	}

	if totalWords == 0 {
		return 0.0
	}

	return float64(commonWords*2) / float64(totalWords)
}

func (a *CausalCorrelationAlgorithm) calculateSeverityCorrelation(sev1, sev2 string) float64 {
	severityMap := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
	}

	val1, ok1 := severityMap[strings.ToLower(sev1)]
	val2, ok2 := severityMap[strings.ToLower(sev2)]

	if !ok1 || !ok2 {
		return 0.5
	}

	diff := math.Abs(float64(val1 - val2))
	maxDiff := 3.0
	return 1.0 - (diff / maxDiff)
}

func (a *CausalCorrelationAlgorithm) calculateCausalConfidence(result1, result2 AnalysisResult, strength float64) float64 {
	// Base confidence on strength and additional factors
	confidence := strength

	// Boost confidence if there are timing relationships
	if strings.Contains(result1.Description, "after") || strings.Contains(result2.Description, "after") {
		confidence += 0.2
	}

	// Boost confidence if there are direct references
	if strings.Contains(strings.ToLower(result1.Description), strings.ToLower(result2.Title)) ||
		strings.Contains(strings.ToLower(result2.Description), strings.ToLower(result1.Title)) {
		confidence += 0.3
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

func (a *CausalCorrelationAlgorithm) calculateCausalImpact(result1, result2 AnalysisResult) string {
	severityMap := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
	}

	val1, ok1 := severityMap[strings.ToLower(result1.Severity)]
	val2, ok2 := severityMap[strings.ToLower(result2.Severity)]

	if !ok1 || !ok2 {
		return "medium"
	}

	maxSeverity := math.Max(float64(val1), float64(val2))
	if maxSeverity >= 4 {
		return "high"
	} else if maxSeverity >= 3 {
		return "medium"
	}
	return "low"
}

// TemporalCorrelationAlgorithm finds time-based correlations
type TemporalCorrelationAlgorithm struct{}

func (a *TemporalCorrelationAlgorithm) Name() string {
	return "temporal"
}

func (a *TemporalCorrelationAlgorithm) FindCorrelations(results []AnalysisResult, steps []RemediationStep) []Correlation {
	var correlations []Correlation

	// For temporal correlations, we need timestamp information
	// This is a simplified version that looks for temporal keywords
	for i, result1 := range results {
		for j, result2 := range results {
			if i >= j {
				continue
			}

			if a.hasTemporalRelationship(result1, result2) {
				strength := a.GetStrength(result1, result2)
				if strength > 0.4 {
					correlations = append(correlations, Correlation{
						ID:          uuid.New().String(),
						Type:        CorrelationTemporal,
						Description: fmt.Sprintf("Temporal correlation between %s and %s", result1.Title, result2.Title),
						Strength:    strength,
						Confidence:  strength * 0.8, // Temporal correlations have slightly lower confidence
						Results:     []string{result1.ID, result2.ID},
						Evidence:    []string{"Temporal relationship detected"},
						Impact:      "medium",
					})
				}
			}
		}
	}

	return correlations
}

func (a *TemporalCorrelationAlgorithm) GetStrength(result1, result2 AnalysisResult) float64 {
	// Calculate temporal correlation strength
	keywordStrength := a.calculateTemporalKeywordStrength(result1.Description, result2.Description)
	categoryStrength := a.calculateTemporalCategoryStrength(result1.Category, result2.Category)

	return (keywordStrength + categoryStrength) / 2.0
}

func (a *TemporalCorrelationAlgorithm) hasTemporalRelationship(result1, result2 AnalysisResult) bool {
	temporalKeywords := []string{"after", "before", "during", "concurrent", "simultaneous", "following", "preceding"}

	desc1 := strings.ToLower(result1.Description)
	desc2 := strings.ToLower(result2.Description)

	for _, keyword := range temporalKeywords {
		if strings.Contains(desc1, keyword) || strings.Contains(desc2, keyword) {
			return true
		}
	}

	return false
}

func (a *TemporalCorrelationAlgorithm) calculateTemporalKeywordStrength(desc1, desc2 string) float64 {
	temporalKeywords := []string{"immediately", "shortly", "soon", "later", "earlier", "recently", "suddenly"}

	desc1Lower := strings.ToLower(desc1)
	desc2Lower := strings.ToLower(desc2)

	strength := 0.0
	for _, keyword := range temporalKeywords {
		if strings.Contains(desc1Lower, keyword) || strings.Contains(desc2Lower, keyword) {
			strength += 0.2
		}
	}

	if strength > 1.0 {
		strength = 1.0
	}
	return strength
}

func (a *TemporalCorrelationAlgorithm) calculateTemporalCategoryStrength(cat1, cat2 string) float64 {
	// Some categories are more likely to have temporal relationships
	temporalCategories := []string{"resource", "application", "network"}

	cat1Lower := strings.ToLower(cat1)
	cat2Lower := strings.ToLower(cat2)

	if contains(temporalCategories, cat1Lower) || contains(temporalCategories, cat2Lower) {
		return 0.6
	}
	return 0.3
}

// SpatialCorrelationAlgorithm finds location-based correlations
type SpatialCorrelationAlgorithm struct{}

func (a *SpatialCorrelationAlgorithm) Name() string {
	return "spatial"
}

func (a *SpatialCorrelationAlgorithm) FindCorrelations(results []AnalysisResult, steps []RemediationStep) []Correlation {
	var correlations []Correlation

	// Group results by spatial indicators (namespace, node, service, etc.)
	spatialGroups := make(map[string][]AnalysisResult)

	for _, result := range results {
		spatialKey := a.extractSpatialKey(result)
		if spatialKey != "" {
			spatialGroups[spatialKey] = append(spatialGroups[spatialKey], result)
		}
	}

	// Find correlations within spatial groups
	for spatialKey, groupResults := range spatialGroups {
		if len(groupResults) > 1 {
			for i, result1 := range groupResults {
				for j, result2 := range groupResults {
					if i >= j {
						continue
					}

					strength := a.GetStrength(result1, result2)
					if strength > 0.5 {
						correlations = append(correlations, Correlation{
							ID:          uuid.New().String(),
							Type:        CorrelationSpatial,
							Description: fmt.Sprintf("Issues co-located in %s: %s and %s", spatialKey, result1.Title, result2.Title),
							Strength:    strength,
							Confidence:  strength * 0.9, // Spatial correlations are usually reliable
							Results:     []string{result1.ID, result2.ID},
							Evidence:    []string{fmt.Sprintf("Both issues located in %s", spatialKey)},
							Impact:      "medium",
						})
					}
				}
			}
		}
	}

	return correlations
}

func (a *SpatialCorrelationAlgorithm) GetStrength(result1, result2 AnalysisResult) float64 {
	// Calculate spatial correlation strength
	locationMatch := a.calculateLocationMatch(result1, result2)
	proximityScore := a.calculateProximity(result1, result2)

	return (locationMatch + proximityScore) / 2.0
}

func (a *SpatialCorrelationAlgorithm) extractSpatialKey(result AnalysisResult) string {
	description := strings.ToLower(result.Description)

	// Extract namespace
	if idx := strings.Index(description, "namespace"); idx != -1 {
		words := strings.Fields(description[idx:])
		if len(words) > 1 {
			return "namespace:" + words[1]
		}
	}

	// Extract node
	if idx := strings.Index(description, "node"); idx != -1 {
		words := strings.Fields(description[idx:])
		if len(words) > 1 {
			return "node:" + words[1]
		}
	}

	// Extract service
	if idx := strings.Index(description, "service"); idx != -1 {
		words := strings.Fields(description[idx:])
		if len(words) > 1 {
			return "service:" + words[1]
		}
	}

	return ""
}

func (a *SpatialCorrelationAlgorithm) calculateLocationMatch(result1, result2 AnalysisResult) float64 {
	key1 := a.extractSpatialKey(result1)
	key2 := a.extractSpatialKey(result2)

	if key1 == "" || key2 == "" {
		return 0.3 // Unknown location
	}

	if key1 == key2 {
		return 1.0 // Same location
	}

	// Check if they're related locations (e.g., same namespace different services)
	parts1 := strings.Split(key1, ":")
	parts2 := strings.Split(key2, ":")

	if len(parts1) == 2 && len(parts2) == 2 && parts1[0] == parts2[0] {
		return 0.7 // Same location type
	}

	return 0.1 // Different locations
}

func (a *SpatialCorrelationAlgorithm) calculateProximity(result1, result2 AnalysisResult) float64 {
	// Simple proximity calculation based on category and description similarity
	categoryMatch := 0.0
	if result1.Category == result2.Category {
		categoryMatch = 0.5
	}

	keywordSimilarity := calculateTextSimilarity(result1.Description, result2.Description)

	return (categoryMatch + keywordSimilarity) / 2.0
}

// FunctionalCorrelationAlgorithm finds function-based correlations
type FunctionalCorrelationAlgorithm struct{}

func (a *FunctionalCorrelationAlgorithm) Name() string {
	return "functional"
}

func (a *FunctionalCorrelationAlgorithm) FindCorrelations(results []AnalysisResult, steps []RemediationStep) []Correlation {
	var correlations []Correlation

	// Group results by functional area
	functionalGroups := make(map[string][]AnalysisResult)

	for _, result := range results {
		functionalArea := a.determineFunctionalArea(result)
		functionalGroups[functionalArea] = append(functionalGroups[functionalArea], result)
	}

	// Find correlations within functional groups
	for area, groupResults := range functionalGroups {
		if len(groupResults) > 1 {
			for i, result1 := range groupResults {
				for j, result2 := range groupResults {
					if i >= j {
						continue
					}

					strength := a.GetStrength(result1, result2)
					if strength > 0.6 {
						correlations = append(correlations, Correlation{
							ID:          uuid.New().String(),
							Type:        CorrelationFunctional,
							Description: fmt.Sprintf("Functional correlation in %s: %s and %s", area, result1.Title, result2.Title),
							Strength:    strength,
							Confidence:  strength * 0.85,
							Results:     []string{result1.ID, result2.ID},
							Evidence:    []string{fmt.Sprintf("Both issues affect %s functionality", area)},
							Impact:      "high",
						})
					}
				}
			}
		}
	}

	return correlations
}

func (a *FunctionalCorrelationAlgorithm) GetStrength(result1, result2 AnalysisResult) float64 {
	// Calculate functional correlation strength
	functionalMatch := a.calculateFunctionalMatch(result1, result2)
	impactCorrelation := a.calculateImpactCorrelation(result1, result2)

	return (functionalMatch + impactCorrelation) / 2.0
}

func (a *FunctionalCorrelationAlgorithm) determineFunctionalArea(result AnalysisResult) string {
	description := strings.ToLower(result.Description)
	category := strings.ToLower(result.Category)

	functionalKeywords := map[string][]string{
		"authentication":  {"auth", "login", "user", "credential", "token"},
		"data_processing": {"process", "compute", "calculation", "transform"},
		"communication":   {"network", "connection", "endpoint", "service"},
		"storage":         {"storage", "disk", "volume", "persistence", "database"},
		"monitoring":      {"log", "metric", "alert", "monitor", "observability"},
		"security":        {"security", "permission", "access", "policy", "rbac"},
	}

	for area, keywords := range functionalKeywords {
		for _, keyword := range keywords {
			if strings.Contains(description, keyword) || strings.Contains(category, keyword) {
				return area
			}
		}
	}

	return "general"
}

func (a *FunctionalCorrelationAlgorithm) calculateFunctionalMatch(result1, result2 AnalysisResult) float64 {
	area1 := a.determineFunctionalArea(result1)
	area2 := a.determineFunctionalArea(result2)

	if area1 == area2 {
		return 1.0
	}

	// Define functional area relationships
	areaRelations := map[string][]string{
		"authentication":  {"security", "communication"},
		"data_processing": {"storage", "communication"},
		"communication":   {"authentication", "security", "monitoring"},
		"storage":         {"data_processing", "monitoring"},
		"monitoring":      {"communication", "storage", "security"},
		"security":        {"authentication", "communication"},
	}

	if relations, exists := areaRelations[area1]; exists {
		for _, related := range relations {
			if area2 == related {
				return 0.7
			}
		}
	}

	return 0.2
}

func (a *FunctionalCorrelationAlgorithm) calculateImpactCorrelation(result1, result2 AnalysisResult) float64 {
	// Both high severity issues in the same functional area have high correlation
	if strings.ToLower(result1.Severity) == "high" && strings.ToLower(result2.Severity) == "high" {
		return 0.9
	}
	if strings.ToLower(result1.Severity) == "critical" || strings.ToLower(result2.Severity) == "critical" {
		return 0.8
	}
	return 0.5
}

// ResourceCorrelationAlgorithm finds resource-based correlations
type ResourceCorrelationAlgorithm struct{}

func (a *ResourceCorrelationAlgorithm) Name() string {
	return "resource"
}

func (a *ResourceCorrelationAlgorithm) FindCorrelations(results []AnalysisResult, steps []RemediationStep) []Correlation {
	var correlations []Correlation

	// Find resource-related results
	resourceResults := make([]AnalysisResult, 0)
	for _, result := range results {
		if a.isResourceRelated(result) {
			resourceResults = append(resourceResults, result)
		}
	}

	// Find correlations between resource issues
	for i, result1 := range resourceResults {
		for j, result2 := range resourceResults {
			if i >= j {
				continue
			}

			strength := a.GetStrength(result1, result2)
			if strength > 0.5 {
				correlations = append(correlations, Correlation{
					ID:          uuid.New().String(),
					Type:        CorrelationResource,
					Description: fmt.Sprintf("Resource correlation: %s and %s", result1.Title, result2.Title),
					Strength:    strength,
					Confidence:  strength * 0.9,
					Results:     []string{result1.ID, result2.ID},
					Evidence:    []string{"Both issues relate to resource usage or availability"},
					Impact:      "high",
				})
			}
		}
	}

	return correlations
}

func (a *ResourceCorrelationAlgorithm) GetStrength(result1, result2 AnalysisResult) float64 {
	resourceType1 := a.extractResourceType(result1)
	resourceType2 := a.extractResourceType(result2)

	if resourceType1 == resourceType2 {
		return 0.9 // Same resource type
	}

	// Define resource type relationships
	resourceRelations := map[string][]string{
		"cpu":         {"memory", "performance"},
		"memory":      {"cpu", "performance", "storage"},
		"storage":     {"memory", "performance"},
		"network":     {"performance", "connectivity"},
		"performance": {"cpu", "memory", "storage", "network"},
	}

	if relations, exists := resourceRelations[resourceType1]; exists {
		for _, related := range relations {
			if resourceType2 == related {
				return 0.7
			}
		}
	}

	return 0.3
}

func (a *ResourceCorrelationAlgorithm) isResourceRelated(result AnalysisResult) bool {
	resourceKeywords := []string{"cpu", "memory", "disk", "storage", "network", "bandwidth", "performance", "resource", "capacity", "usage", "utilization"}

	description := strings.ToLower(result.Description)
	category := strings.ToLower(result.Category)
	title := strings.ToLower(result.Title)

	for _, keyword := range resourceKeywords {
		if strings.Contains(description, keyword) || strings.Contains(category, keyword) || strings.Contains(title, keyword) {
			return true
		}
	}

	return false
}

func (a *ResourceCorrelationAlgorithm) extractResourceType(result AnalysisResult) string {
	text := strings.ToLower(result.Description + " " + result.Title + " " + result.Category)

	resourceTypes := []string{"cpu", "memory", "storage", "network", "performance"}
	for _, resourceType := range resourceTypes {
		if strings.Contains(text, resourceType) {
			return resourceType
		}
	}

	return "general"
}

// Helper methods for CorrelationEngine

func (ce *CorrelationEngine) filterCorrelations(correlations []Correlation) []Correlation {
	var filtered []Correlation

	for _, correlation := range correlations {
		if correlation.Strength >= ce.thresholds.MinStrength &&
			correlation.Confidence >= ce.thresholds.MinConfidence {
			filtered = append(filtered, correlation)
		}
	}

	return filtered
}

func (ce *CorrelationEngine) rankCorrelations(correlations []Correlation) []Correlation {
	// Sort by combined score (strength * confidence)
	sort.Slice(correlations, func(i, j int) bool {
		scoreI := correlations[i].Strength * correlations[i].Confidence
		scoreJ := correlations[j].Strength * correlations[j].Confidence
		return scoreI > scoreJ
	})

	return correlations
}

func (ce *CorrelationEngine) generateCacheKey(results []AnalysisResult, steps []RemediationStep) string {
	// Generate a simple cache key based on result and step IDs
	var keyParts []string

	for _, result := range results {
		keyParts = append(keyParts, result.ID)
	}
	for _, step := range steps {
		keyParts = append(keyParts, step.ID)
	}

	return strings.Join(keyParts, "|")
}

// CorrelationCache methods

func (cc *CorrelationCache) Get(key string) (CacheEntry, bool) {
	if entry, exists := cc.entries[key]; exists {
		// Check TTL
		if time.Since(entry.Timestamp) < cc.ttl {
			return entry, true
		}
		// Remove expired entry
		delete(cc.entries, key)
	}
	return CacheEntry{}, false
}

func (cc *CorrelationCache) Put(key string, entry CacheEntry) {
	// Clean up expired entries if at capacity
	if len(cc.entries) >= cc.maxSize {
		cc.cleanup()
	}

	cc.entries[key] = entry
}

func (cc *CorrelationCache) cleanup() {
	now := time.Now()
	for key, entry := range cc.entries {
		if now.Sub(entry.Timestamp) > cc.ttl {
			delete(cc.entries, key)
		}
	}
}

// Utility functions

func calculateTextSimilarity(text1, text2 string) float64 {
	words1 := strings.Fields(strings.ToLower(text1))
	words2 := strings.Fields(strings.ToLower(text2))

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	commonWords := 0
	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 && len(word1) > 2 {
				commonWords++
				break
			}
		}
	}

	// Jaccard similarity coefficient
	totalWords := len(words1) + len(words2) - commonWords
	if totalWords == 0 {
		return 0.0
	}

	return float64(commonWords) / float64(totalWords)
}
