package remediation

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

)

// TrendAnalyzer performs trend analysis and historical comparison on analysis data
type TrendAnalyzer struct {
	historyStore HistoryStore
	options      TrendAnalysisOptions
}

// HistoryStore interface for storing and retrieving historical analysis data
type HistoryStore interface {
	Store(result HistoricalAnalysisResult) error
	GetByTimeRange(start, end time.Time) ([]HistoricalAnalysisResult, error)
	GetByAnalyzer(analyzerName string, limit int) ([]HistoricalAnalysisResult, error)
	GetLatest(limit int) ([]HistoricalAnalysisResult, error)
	Search(query HistoryQuery) ([]HistoricalAnalysisResult, error)
}

// EnvironmentInfo contains information about the environment where analysis was performed
type EnvironmentInfo struct {
	ClusterVersion string            `json:"cluster_version"`
	NodeCount      int               `json:"node_count"`
	Namespace      string            `json:"namespace"`
	Labels         map[string]string `json:"labels"`
	Platform       string            `json:"platform"`
	CloudProvider  string            `json:"cloud_provider"`
	Region         string            `json:"region"`
	Version        string            `json:"version"`
}

// AnalysisMetadata contains metadata about the analysis
type AnalysisMetadata struct {
	Version     string            `json:"version"`
	Source      string            `json:"source"`
	Tags        []string          `json:"tags"`
	Annotations map[string]string `json:"annotations"`
}


// InsightType represents the type of insight
type InsightType string

const (
	InsightTypePerformance InsightType = "performance"
	InsightTypeReliability InsightType = "reliability"
	InsightTypeSecurity    InsightType = "security"
	InsightTypeResource    InsightType = "resource"
	InsightTypeTrend       InsightType = "trend"
)

// HistoricalAnalysisResult represents a historical analysis result
type HistoricalAnalysisResult struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	Environment  EnvironmentInfo   `json:"environment"`
	Results      []string          `json:"results"` // Changed from v1beta2.AnalyzeResult to string
	Remediation  []RemediationStep `json:"remediation,omitempty"`
	Metadata     AnalysisMetadata  `json:"metadata"`
	Duration     time.Duration     `json:"duration"`
	Success      bool              `json:"success"`
	ErrorMessage string            `json:"error_message,omitempty"`
}

// HistoryQuery defines query parameters for searching historical data
type HistoryQuery struct {
	TimeRange   *TimeRange         `json:"time_range,omitempty"`
	Environment *EnvironmentFilter `json:"environment,omitempty"`
	Analyzers   []string           `json:"analyzers,omitempty"`
	Status      []string           `json:"status,omitempty"`
	Success     *bool              `json:"success,omitempty"`
	Limit       int                `json:"limit,omitempty"`
	Offset      int                `json:"offset,omitempty"`
}

// TimeRange defines a time period for queries
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// EnvironmentFilter defines environment filtering criteria
type EnvironmentFilter struct {
	Platform      string   `json:"platform,omitempty"`
	Version       string   `json:"version,omitempty"`
	CloudProvider string   `json:"cloud_provider,omitempty"`
	Region        string   `json:"region,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

// TrendAnalysisOptions defines options for trend analysis
type TrendAnalysisOptions struct {
	TimeWindow        time.Duration `json:"time_window"`        // Time window for analysis
	MinDataPoints     int           `json:"min_data_points"`    // Minimum data points required
	ConfidenceLevel   float64       `json:"confidence_level"`   // Statistical confidence level
	SeasonalityWindow time.Duration `json:"seasonality_window"` // Window for seasonality detection
	OutlierThreshold  float64       `json:"outlier_threshold"`  // Threshold for outlier detection
	TrendSensitivity  float64       `json:"trend_sensitivity"`  // Sensitivity for trend detection
}

// TrendAnalysisResult contains the results of trend analysis
type TrendAnalysisResult struct {
	Summary         TrendSummary           `json:"summary"`
	AnalyzerTrends  []AnalyzerTrend        `json:"analyzer_trends"`
	SystemTrends    SystemTrend            `json:"system_trends"`
	Recommendations []TrendRecommendation  `json:"recommendations"`
	Comparisons     []HistoricalComparison `json:"comparisons"`
	Insights        []TrendInsight         `json:"insights"`
	Metadata        TrendAnalysisMetadata  `json:"metadata"`
}

// TrendSummary provides overall trend information
type TrendSummary struct {
	TimeRange       TimeRange          `json:"time_range"`
	TotalAnalyses   int                `json:"total_analyses"`
	DataPoints      int                `json:"data_points"`
	OverallTrend    TrendDirection     `json:"overall_trend"`
	TrendConfidence float64            `json:"trend_confidence"`
	ChangeRate      float64            `json:"change_rate"` // Rate of change
	Seasonality     SeasonalityInfo    `json:"seasonality"`
	Anomalies       []AnomalyDetection `json:"anomalies"`
	HealthScore     float64            `json:"health_score"` // Overall system health score
	HealthTrend     TrendDirection     `json:"health_trend"`
}

// AnalyzerTrend represents trend information for a specific analyzer
type AnalyzerTrend struct {
	AnalyzerName    string              `json:"analyzer_name"`
	Trend           TrendDirection      `json:"trend"`
	Confidence      float64             `json:"confidence"`
	DataPoints      int                 `json:"data_points"`
	FailureRate     TrendMetric         `json:"failure_rate"`
	PerformanceTime TrendMetric         `json:"performance_time"`
	Seasonality     SeasonalityInfo     `json:"seasonality"`
	Predictions     []TrendPrediction   `json:"predictions"`
	RecentChanges   []SignificantChange `json:"recent_changes"`
}

// SystemTrend represents overall system trend information
type SystemTrend struct {
	OverallHealth    TrendMetric           `json:"overall_health"`
	CriticalIssues   TrendMetric           `json:"critical_issues"`
	WarningIssues    TrendMetric           `json:"warning_issues"`
	PassingChecks    TrendMetric           `json:"passing_checks"`
	RemediationRate  TrendMetric           `json:"remediation_rate"`
	EnvironmentStats EnvironmentTrendStats `json:"environment_stats"`
}

// TrendMetric represents a trending metric with statistical information
type TrendMetric struct {
	Current       float64         `json:"current"`
	Previous      float64         `json:"previous"`
	Change        float64         `json:"change"`         // Absolute change
	ChangePercent float64         `json:"change_percent"` // Percentage change
	Trend         TrendDirection  `json:"trend"`
	Confidence    float64         `json:"confidence"`
	Statistics    TrendStatistics `json:"statistics"`
}

// TrendStatistics provides statistical information about a trend
type TrendStatistics struct {
	Mean     float64 `json:"mean"`
	Median   float64 `json:"median"`
	StdDev   float64 `json:"std_dev"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Variance float64 `json:"variance"`
	Skewness float64 `json:"skewness"` // Measure of asymmetry
	Kurtosis float64 `json:"kurtosis"` // Measure of tail heaviness
}

// SeasonalityInfo contains information about seasonal patterns
type SeasonalityInfo struct {
	HasSeasonality bool              `json:"has_seasonality"`
	Period         time.Duration     `json:"period,omitempty"`
	Strength       float64           `json:"strength,omitempty"` // 0-1, strength of seasonal pattern
	Patterns       []SeasonalPattern `json:"patterns,omitempty"`
}

// SeasonalPattern represents a detected seasonal pattern
type SeasonalPattern struct {
	Type       string        `json:"type"` // "daily", "weekly", "monthly"
	Period     time.Duration `json:"period"`
	Amplitude  float64       `json:"amplitude"` // Strength of the pattern
	Phase      time.Duration `json:"phase"`     // Phase offset
	Confidence float64       `json:"confidence"`
}

// AnomalyDetection represents a detected anomaly in the data
type AnomalyDetection struct {
	Timestamp     time.Time   `json:"timestamp"`
	Analyzer      string      `json:"analyzer,omitempty"`
	Type          AnomalyType `json:"type"`
	Severity      float64     `json:"severity"` // 0-1, severity of anomaly
	Description   string      `json:"description"`
	ExpectedValue float64     `json:"expected_value"`
	ActualValue   float64     `json:"actual_value"`
	Confidence    float64     `json:"confidence"`
	Context       string      `json:"context,omitempty"`
}

// AnomalyType represents the type of anomaly detected
type AnomalyType string

const (
	AnomalySpike      AnomalyType = "spike"       // Sudden increase
	AnomalyDrop       AnomalyType = "drop"        // Sudden decrease
	AnomalyOutlier    AnomalyType = "outlier"     // Statistical outlier
	AnomalyTrendBreak AnomalyType = "trend_break" // Break in established trend
	AnomalyMissing    AnomalyType = "missing"     // Missing expected data
)

// TrendPrediction represents a prediction based on trend analysis
type TrendPrediction struct {
	Timestamp       time.Time `json:"timestamp"`
	PredictedValue  float64   `json:"predicted_value"`
	ConfidenceRange struct {
		Lower float64 `json:"lower"`
		Upper float64 `json:"upper"`
	} `json:"confidence_range"`
	Confidence  float64  `json:"confidence"`
	Method      string   `json:"method"` // Prediction method used
	Assumptions []string `json:"assumptions"`
}

// SignificantChange represents a significant change in analyzer behavior
type SignificantChange struct {
	Timestamp   time.Time `json:"timestamp"`
	ChangeType  string    `json:"change_type"` // "improvement", "degradation", "volatility"
	Magnitude   float64   `json:"magnitude"`
	Description string    `json:"description"`
	Impact      string    `json:"impact"` // "low", "medium", "high"
	Confidence  float64   `json:"confidence"`
}

// TrendRecommendation provides recommendations based on trend analysis
type TrendRecommendation struct {
	ID          string                  `json:"id"`
	Type        TrendRecommendationType `json:"type"`
	Priority    RemediationPriority     `json:"priority"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Impact      string                  `json:"impact"`
	Confidence  float64                 `json:"confidence"`
	Actions     []string                `json:"actions"`
	Evidence    []string                `json:"evidence"`
	Timeline    string                  `json:"timeline"`
}

// TrendRecommendationType represents types of trend-based recommendations
type TrendRecommendationType string

const (
	RecommendationPreventive    TrendRecommendationType = "preventive"
	RecommendationOptimization  TrendRecommendationType = "optimization"
	RecommendationInvestigation TrendRecommendationType = "investigation"
	RecommendationMonitoring    TrendRecommendationType = "monitoring"
	RecommendationRemoval       TrendRecommendationType = "removal"
)

// HistoricalComparison compares current analysis with historical data
type HistoricalComparison struct {
	ComparisonType    ComparisonType     `json:"comparison_type"`
	BaselinePeriod    TimeRange          `json:"baseline_period"`
	CurrentPeriod     TimeRange          `json:"current_period"`
	MetricComparisons []MetricComparison `json:"metric_comparisons"`
	Summary           string             `json:"summary"`
	Significance      float64            `json:"significance"` // Statistical significance
}

// ComparisonType represents the type of historical comparison
type ComparisonType string

const (
	ComparisonPeriodOverPeriod ComparisonType = "period_over_period"
	ComparisonYearOverYear     ComparisonType = "year_over_year"
	ComparisonBaseline         ComparisonType = "baseline"
	ComparisonMovingAverage    ComparisonType = "moving_average"
)

// MetricComparison represents a comparison of a specific metric
type MetricComparison struct {
	MetricName     string         `json:"metric_name"`
	BaselineValue  float64        `json:"baseline_value"`
	CurrentValue   float64        `json:"current_value"`
	Change         float64        `json:"change"`
	ChangePercent  float64        `json:"change_percent"`
	Direction      TrendDirection `json:"direction"`
	Significance   float64        `json:"significance"`
	Interpretation string         `json:"interpretation"`
}

// TrendInsight represents an insight derived from trend analysis
type TrendInsight struct {
	ID               string      `json:"id"`
	Type             InsightType `json:"type"`
	Category         string      `json:"category"`
	Title            string      `json:"title"`
	Description      string      `json:"description"`
	Confidence       float64     `json:"confidence"`
	Severity         string      `json:"severity"`
	Tags             []string    `json:"tags"`
	RelatedAnalyzers []string    `json:"related_analyzers"`
	Evidence         []string    `json:"evidence"`
	Timestamp        time.Time   `json:"timestamp"`
}

// EnvironmentTrendStats provides trend statistics by environment
type EnvironmentTrendStats struct {
	ByPlatform      map[string]TrendMetric `json:"by_platform"`
	ByCloudProvider map[string]TrendMetric `json:"by_cloud_provider"`
	ByRegion        map[string]TrendMetric `json:"by_region"`
	ByVersion       map[string]TrendMetric `json:"by_version"`
}

// TrendAnalysisMetadata contains metadata about the trend analysis
type TrendAnalysisMetadata struct {
	AnalysisTime   time.Time `json:"analysis_time"`
	Version        string    `json:"version"`
	DataSources    []string  `json:"data_sources"`
	Methods        []string  `json:"methods"`
	Limitations    []string  `json:"limitations"`
	Assumptions    []string  `json:"assumptions"`
	QualityMetrics struct {
		DataCompleteness float64 `json:"data_completeness"`
		DataQuality      float64 `json:"data_quality"`
		StatisticalPower float64 `json:"statistical_power"`
	} `json:"quality_metrics"`
}

// NewTrendAnalyzer creates a new trend analyzer
func NewTrendAnalyzer(historyStore HistoryStore) *TrendAnalyzer {
	return &TrendAnalyzer{
		historyStore: historyStore,
		options: TrendAnalysisOptions{
			TimeWindow:        24 * time.Hour * 30, // 30 days default
			MinDataPoints:     5,
			ConfidenceLevel:   0.95,
			SeasonalityWindow: 24 * time.Hour * 7, // 1 week for seasonality
			OutlierThreshold:  2.0,                // 2 standard deviations
			TrendSensitivity:  0.1,                // 10% change threshold
		},
	}
}

// WithOptions sets analysis options
func (ta *TrendAnalyzer) WithOptions(options TrendAnalysisOptions) *TrendAnalyzer {
	ta.options = options
	return ta
}

// AnalyzeTrends performs comprehensive trend analysis
func (ta *TrendAnalyzer) AnalyzeTrends(ctx RemediationContext) (*TrendAnalysisResult, error) {
	// Get historical data
	endTime := time.Now()
	startTime := endTime.Add(-ta.options.TimeWindow)

	historicalData, err := ta.historyStore.GetByTimeRange(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve historical data: %w", err)
	}

	if len(historicalData) < ta.options.MinDataPoints {
		return nil, fmt.Errorf("insufficient data points for trend analysis: %d (minimum: %d)",
			len(historicalData), ta.options.MinDataPoints)
	}

	// Perform analysis
	summary := ta.calculateTrendSummary(historicalData, startTime, endTime)
	analyzerTrends := ta.analyzeAnalyzerTrends(historicalData)
	systemTrends := ta.analyzeSystemTrends(historicalData)
	recommendations := ta.generateTrendRecommendations(analyzerTrends, systemTrends, summary)
	comparisons := ta.performHistoricalComparisons(historicalData)
	insights := ta.generateInsights(analyzerTrends, systemTrends, historicalData)

	return &TrendAnalysisResult{
		Summary:         summary,
		AnalyzerTrends:  analyzerTrends,
		SystemTrends:    systemTrends,
		Recommendations: recommendations,
		Comparisons:     comparisons,
		Insights:        insights,
		Metadata: TrendAnalysisMetadata{
			AnalysisTime: time.Now(),
			Version:      "1.0.0",
			DataSources:  []string{"historical_analysis_results"},
			Methods:      []string{"linear_regression", "seasonal_decomposition", "anomaly_detection"},
			Limitations:  []string{"limited_historical_data", "environmental_changes"},
			Assumptions:  []string{"consistent_environment", "representative_sample"},
			QualityMetrics: struct {
				DataCompleteness float64 `json:"data_completeness"`
				DataQuality      float64 `json:"data_quality"`
				StatisticalPower float64 `json:"statistical_power"`
			}{
				DataCompleteness: ta.calculateDataCompleteness(historicalData),
				DataQuality:      ta.calculateDataQuality(historicalData),
				StatisticalPower: ta.calculateStatisticalPower(len(historicalData)),
			},
		},
	}, nil
}

// calculateTrendSummary calculates overall trend summary
func (ta *TrendAnalyzer) calculateTrendSummary(data []HistoricalAnalysisResult, start, end time.Time) TrendSummary {
	if len(data) == 0 {
		return TrendSummary{
			TimeRange:       TimeRange{Start: start, End: end},
			OverallTrend:    TrendUnknown,
			TrendConfidence: 0.0,
		}
	}

	// Sort data by timestamp
	sort.Slice(data, func(i, j int) bool {
		return data[i].Timestamp.Before(data[j].Timestamp)
	})

	// Calculate health scores over time
	healthScores := make([]float64, len(data))
	timestamps := make([]time.Time, len(data))

	for i, result := range data {
		healthScores[i] = ta.calculateHealthScore(result.Results)
		timestamps[i] = result.Timestamp
	}

	// Detect overall trend
	overallTrend, confidence := ta.detectTrend(healthScores, timestamps)
	changeRate := ta.calculateChangeRate(healthScores)

	// Detect seasonality
	seasonality := ta.detectSeasonality(healthScores, timestamps)

	// Detect anomalies
	anomalies := ta.detectAnomalies(healthScores, timestamps, data)

	// Calculate current health score
	currentHealthScore := healthScores[len(healthScores)-1]

	// Determine health trend
	healthTrend := overallTrend
	if len(healthScores) >= 2 {
		recent := healthScores[len(healthScores)-2:]
		if recent[1] > recent[0] {
			healthTrend = TrendImproving
		} else if recent[1] < recent[0] {
			healthTrend = TrendDegrading
		} else {
			healthTrend = TrendStable
		}
	}

	return TrendSummary{
		TimeRange:       TimeRange{Start: start, End: end},
		TotalAnalyses:   len(data),
		DataPoints:      len(healthScores),
		OverallTrend:    overallTrend,
		TrendConfidence: confidence,
		ChangeRate:      changeRate,
		Seasonality:     seasonality,
		Anomalies:       anomalies,
		HealthScore:     currentHealthScore,
		HealthTrend:     healthTrend,
	}
}

// analyzeAnalyzerTrends analyzes trends for individual analyzers
func (ta *TrendAnalyzer) analyzeAnalyzerTrends(data []HistoricalAnalysisResult) []AnalyzerTrend {
	// Group data by analyzer
	analyzerData := make(map[string][]HistoricalAnalysisResult)
	for _, result := range data {
		for _, analyzeResult := range result.Results {
			analyzerName := ta.getAnalyzerName(analyzeResult)
			analyzerData[analyzerName] = append(analyzerData[analyzerName], result)
		}
	}

	var trends []AnalyzerTrend
	for analyzerName, results := range analyzerData {
		if len(results) < ta.options.MinDataPoints {
			continue
		}

		trend := ta.analyzeAnalyzerTrend(analyzerName, results)
		trends = append(trends, trend)
	}

	// Sort by confidence (most reliable trends first)
	sort.Slice(trends, func(i, j int) bool {
		return trends[i].Confidence > trends[j].Confidence
	})

	return trends
}

// analyzeAnalyzerTrend analyzes trend for a specific analyzer
func (ta *TrendAnalyzer) analyzeAnalyzerTrend(analyzerName string, data []HistoricalAnalysisResult) AnalyzerTrend {
	// Extract metrics over time
	timestamps := make([]time.Time, len(data))
	failureRates := make([]float64, len(data))
	performanceTimes := make([]float64, len(data))

	for i, result := range data {
		timestamps[i] = result.Timestamp
		failureRates[i] = ta.calculateAnalyzerFailureRate(analyzerName, result.Results)
		performanceTimes[i] = float64(result.Duration.Milliseconds())
	}

	// Detect trends
	failureTrend, failureConfidence := ta.detectTrend(failureRates, timestamps)
	performanceTrend, performanceConfidence := ta.detectTrend(performanceTimes, timestamps)

	// Overall confidence is average of individual confidences
	overallConfidence := (failureConfidence + performanceConfidence) / 2

	// Determine overall trend (failure trend is more important)
	overallTrend := failureTrend
	if failureConfidence < performanceConfidence {
		overallTrend = performanceTrend
	}

	// Detect seasonality
	seasonality := ta.detectSeasonality(failureRates, timestamps)

	// Generate predictions
	predictions := ta.generatePredictions(failureRates, timestamps, 7) // 7 days ahead

	// Detect significant changes
	changes := ta.detectSignificantChanges(failureRates, timestamps)

	return AnalyzerTrend{
		AnalyzerName: analyzerName,
		Trend:        overallTrend,
		Confidence:   overallConfidence,
		DataPoints:   len(data),
		FailureRate: TrendMetric{
			Current:       failureRates[len(failureRates)-1],
			Previous:      ta.getPreviousValue(failureRates),
			Change:        ta.calculateAbsoluteChange(failureRates),
			ChangePercent: ta.calculatePercentageChange(failureRates),
			Trend:         failureTrend,
			Confidence:    failureConfidence,
			Statistics:    ta.calculateStatistics(failureRates),
		},
		PerformanceTime: TrendMetric{
			Current:       performanceTimes[len(performanceTimes)-1],
			Previous:      ta.getPreviousValue(performanceTimes),
			Change:        ta.calculateAbsoluteChange(performanceTimes),
			ChangePercent: ta.calculatePercentageChange(performanceTimes),
			Trend:         performanceTrend,
			Confidence:    performanceConfidence,
			Statistics:    ta.calculateStatistics(performanceTimes),
		},
		Seasonality:   seasonality,
		Predictions:   predictions,
		RecentChanges: changes,
	}
}

// analyzeSystemTrends analyzes overall system trends
func (ta *TrendAnalyzer) analyzeSystemTrends(data []HistoricalAnalysisResult) SystemTrend {
	if len(data) == 0 {
		return SystemTrend{}
	}

	// Extract system-level metrics over time
	timestamps := make([]time.Time, len(data))
	healthScores := make([]float64, len(data))
	criticalCounts := make([]float64, len(data))
	warningCounts := make([]float64, len(data))
	passingCounts := make([]float64, len(data))
	remediationRates := make([]float64, len(data))

	for i, result := range data {
		timestamps[i] = result.Timestamp
		healthScores[i] = ta.calculateHealthScore(result.Results)
		criticalCounts[i] = float64(ta.countByStatus(result.Results, "fail"))
		warningCounts[i] = float64(ta.countByStatus(result.Results, "warn"))
		passingCounts[i] = float64(ta.countByStatus(result.Results, "pass"))
		remediationRates[i] = ta.calculateRemediationRate(result.Remediation)
	}

	// Analyze trends for each metric
	healthTrend, healthConfidence := ta.detectTrend(healthScores, timestamps)
	criticalTrend, criticalConfidence := ta.detectTrend(criticalCounts, timestamps)
	warningTrend, warningConfidence := ta.detectTrend(warningCounts, timestamps)
	passingTrend, passingConfidence := ta.detectTrend(passingCounts, timestamps)
	remediationTrendDir, remediationConfidence := ta.detectTrend(remediationRates, timestamps)

	// Calculate environment statistics
	envStats := ta.calculateEnvironmentTrendStats(data)

	return SystemTrend{
		OverallHealth: TrendMetric{
			Current:       healthScores[len(healthScores)-1],
			Previous:      ta.getPreviousValue(healthScores),
			Change:        ta.calculateAbsoluteChange(healthScores),
			ChangePercent: ta.calculatePercentageChange(healthScores),
			Trend:         healthTrend,
			Confidence:    healthConfidence,
			Statistics:    ta.calculateStatistics(healthScores),
		},
		CriticalIssues: TrendMetric{
			Current:       criticalCounts[len(criticalCounts)-1],
			Previous:      ta.getPreviousValue(criticalCounts),
			Change:        ta.calculateAbsoluteChange(criticalCounts),
			ChangePercent: ta.calculatePercentageChange(criticalCounts),
			Trend:         criticalTrend,
			Confidence:    criticalConfidence,
			Statistics:    ta.calculateStatistics(criticalCounts),
		},
		WarningIssues: TrendMetric{
			Current:       warningCounts[len(warningCounts)-1],
			Previous:      ta.getPreviousValue(warningCounts),
			Change:        ta.calculateAbsoluteChange(warningCounts),
			ChangePercent: ta.calculatePercentageChange(warningCounts),
			Trend:         warningTrend,
			Confidence:    warningConfidence,
			Statistics:    ta.calculateStatistics(warningCounts),
		},
		PassingChecks: TrendMetric{
			Current:       passingCounts[len(passingCounts)-1],
			Previous:      ta.getPreviousValue(passingCounts),
			Change:        ta.calculateAbsoluteChange(passingCounts),
			ChangePercent: ta.calculatePercentageChange(passingCounts),
			Trend:         passingTrend,
			Confidence:    passingConfidence,
			Statistics:    ta.calculateStatistics(passingCounts),
		},
		RemediationRate: TrendMetric{
			Current:       remediationRates[len(remediationRates)-1],
			Previous:      ta.getPreviousValue(remediationRates),
			Change:        ta.calculateAbsoluteChange(remediationRates),
			ChangePercent: ta.calculatePercentageChange(remediationRates),
			Trend:         remediationTrendDir,
			Confidence:    remediationConfidence,
			Statistics:    ta.calculateStatistics(remediationRates),
		},
		EnvironmentStats: envStats,
	}
}

// Helper methods for trend analysis

func (ta *TrendAnalyzer) calculateHealthScore(results []string) float64 {
	if len(results) == 0 {
		return 1.0
	}

	totalWeight := 0.0
	weightedScore := 0.0

	for _, result := range results {
		weight := 1.0

		// Weight critical issues more heavily
		if strings.Contains(strings.ToLower(result), "warn") {
			weight = 2.0
		}
		if strings.Contains(strings.ToLower(result), "fail") {
			weight = 3.0
		}

		var score float64
		if strings.Contains(strings.ToLower(result), "pass") {
			score = 1.0
		} else if strings.Contains(strings.ToLower(result), "warn") {
			score = 0.5
		} else if strings.Contains(strings.ToLower(result), "fail") {
			score = 0.0
		} else {
			score = 0.5 // Unknown status
		}

		weightedScore += score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 1.0
	}

	return weightedScore / totalWeight
}

func (ta *TrendAnalyzer) getAnalyzerName(result string) string {
	// Since result is now a string, just return it or a default
	if result != "" {
		return result
	}
	return "unknown"
}

func (ta *TrendAnalyzer) calculateAnalyzerFailureRate(analyzerName string, results []string) float64 {
	total := 0
	failures := 0

	for _, result := range results {
		if ta.getAnalyzerName(result) == analyzerName {
			total++
			if strings.Contains(strings.ToLower(result), "fail") {
				failures++
			}
		}
	}

	if total == 0 {
		return 0.0
	}

	return float64(failures) / float64(total)
}

func (ta *TrendAnalyzer) countByStatus(results []string, status string) int {
	count := 0
	for _, result := range results {
		// Since result is now a string, do a simple string comparison
		if strings.Contains(strings.ToLower(result), strings.ToLower(status)) {
			count++
		}
	}
	return count
}

func (ta *TrendAnalyzer) calculateRemediationRate(remediation []RemediationStep) float64 {
	if len(remediation) == 0 {
		return 0.0
	}

	// Simple heuristic: higher priority and lower difficulty steps have higher success rates
	totalWeight := 0.0
	weightedSuccess := 0.0

	for _, step := range remediation {
		weight := 1.0
		successRate := 0.7 // Base success rate

		// Adjust based on priority
		switch step.Priority {
		case PriorityCritical:
			weight = 3.0
			successRate = 0.8
		case PriorityHigh:
			weight = 2.0
			successRate = 0.75
		case PriorityMedium:
			weight = 1.0
			successRate = 0.7
		case PriorityLow:
			weight = 0.5
			successRate = 0.65
		}

		// Adjust based on difficulty
		switch step.Difficulty {
		case DifficultyEasy:
			successRate += 0.2
		case DifficultyModerate:
			successRate += 0.1
		case DifficultyHard:
			successRate -= 0.1
		case DifficultyExpert:
			successRate -= 0.2
		}

		if successRate > 1.0 {
			successRate = 1.0
		}
		if successRate < 0.0 {
			successRate = 0.0
		}

		weightedSuccess += successRate * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0.0
	}

	return weightedSuccess / totalWeight
}

func (ta *TrendAnalyzer) detectTrend(values []float64, timestamps []time.Time) (TrendDirection, float64) {
	if len(values) < 2 {
		return TrendUnknown, 0.0
	}

	// Simple linear regression to detect trend
	n := float64(len(values))

	// Convert timestamps to numeric values (hours since first timestamp)
	x := make([]float64, len(timestamps))
	base := timestamps[0]
	for i, ts := range timestamps {
		x[i] = ts.Sub(base).Hours()
	}

	// Calculate means
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i := 0; i < len(values); i++ {
		sumX += x[i]
		sumY += values[i]
		sumXY += x[i] * values[i]
		sumX2 += x[i] * x[i]
	}

	meanX := sumX / n
	meanY := sumY / n

	// Calculate slope (trend)
	numerator := sumXY - n*meanX*meanY
	denominator := sumX2 - n*meanX*meanX

	if math.Abs(denominator) < 1e-10 {
		return TrendStable, 0.0
	}

	slope := numerator / denominator

	// Calculate correlation coefficient for confidence
	sumY2 := 0.0
	for _, y := range values {
		sumY2 += y * y
	}

	r2Numerator := numerator * numerator
	r2Denominator := (sumX2 - n*meanX*meanX) * (sumY2 - n*meanY*meanY)

	var confidence float64
	if r2Denominator > 0 {
		r2 := r2Numerator / r2Denominator
		confidence = math.Sqrt(r2) // R-squared as confidence measure
	}

	// Determine trend direction based on slope and sensitivity
	slopeThreshold := ta.options.TrendSensitivity * meanY
	if math.Abs(slope) < slopeThreshold {
		return TrendStable, confidence
	} else if slope > 0 {
		return TrendImproving, confidence
	} else {
		return TrendDegrading, confidence
	}
}

func (ta *TrendAnalyzer) calculateChangeRate(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}

	first := values[0]
	last := values[len(values)-1]

	if first == 0 {
		if last == 0 {
			return 0.0
		}
		return 100.0 // Arbitrarily large change
	}

	return (last - first) / first * 100.0
}

func (ta *TrendAnalyzer) detectSeasonality(values []float64, timestamps []time.Time) SeasonalityInfo {
	// Simple seasonality detection - in a real implementation, this would use more sophisticated algorithms
	if len(values) < 7 { // Need at least a week of data
		return SeasonalityInfo{HasSeasonality: false}
	}

	// Check for daily patterns (if we have hourly data)
	// Check for weekly patterns (if we have daily data)
	// This is a simplified implementation

	return SeasonalityInfo{
		HasSeasonality: false, // Placeholder - would implement FFT or autocorrelation analysis
	}
}

func (ta *TrendAnalyzer) detectAnomalies(values []float64, timestamps []time.Time, data []HistoricalAnalysisResult) []AnomalyDetection {
	if len(values) < 3 {
		return nil
	}

	var anomalies []AnomalyDetection

	// Calculate basic statistics
	stats := ta.calculateStatistics(values)
	threshold := ta.options.OutlierThreshold * stats.StdDev

	// Detect outliers
	for i, value := range values {
		if math.Abs(value-stats.Mean) > threshold {
			anomalyType := AnomalyOutlier
			if value > stats.Mean {
				anomalyType = AnomalySpike
			} else {
				anomalyType = AnomalyDrop
			}

			severity := math.Abs(value-stats.Mean) / stats.StdDev / ta.options.OutlierThreshold
			if severity > 1.0 {
				severity = 1.0
			}

			anomalies = append(anomalies, AnomalyDetection{
				Timestamp:     timestamps[i],
				Type:          anomalyType,
				Severity:      severity,
				Description:   fmt.Sprintf("%s detected in health score", anomalyType),
				ExpectedValue: stats.Mean,
				ActualValue:   value,
				Confidence:    0.8, // Placeholder confidence
				Context:       fmt.Sprintf("Analysis at %s", timestamps[i].Format(time.RFC3339)),
			})
		}
	}

	return anomalies
}

func (ta *TrendAnalyzer) generatePredictions(values []float64, timestamps []time.Time, daysAhead int) []TrendPrediction {
	if len(values) < 3 {
		return nil
	}

	// Simple linear extrapolation
	trend, confidence := ta.detectTrend(values, timestamps)
	if trend == TrendStable || confidence < 0.5 {
		return nil // Not enough trend to make predictions
	}

	var predictions []TrendPrediction

	// Calculate daily change rate
	totalDays := timestamps[len(timestamps)-1].Sub(timestamps[0]).Hours() / 24
	if totalDays == 0 {
		return nil
	}

	dailyChange := (values[len(values)-1] - values[0]) / totalDays
	currentValue := values[len(values)-1]

	for day := 1; day <= daysAhead; day++ {
		predictedValue := currentValue + dailyChange*float64(day)
		predictionTime := timestamps[len(timestamps)-1].Add(time.Duration(day) * 24 * time.Hour)

		// Calculate confidence range (simple approach)
		stats := ta.calculateStatistics(values)
		confidenceRange := stats.StdDev * 1.96 // 95% confidence interval

		predictions = append(predictions, TrendPrediction{
			Timestamp:      predictionTime,
			PredictedValue: predictedValue,
			ConfidenceRange: struct {
				Lower float64 `json:"lower"`
				Upper float64 `json:"upper"`
			}{
				Lower: predictedValue - confidenceRange,
				Upper: predictedValue + confidenceRange,
			},
			Confidence:  confidence * 0.8, // Reduce confidence for predictions
			Method:      "linear_extrapolation",
			Assumptions: []string{"trend_continues", "no_external_changes"},
		})
	}

	return predictions
}

func (ta *TrendAnalyzer) detectSignificantChanges(values []float64, timestamps []time.Time) []SignificantChange {
	if len(values) < 3 {
		return nil
	}

	var changes []SignificantChange
	stats := ta.calculateStatistics(values)

	// Look for significant changes between adjacent points
	for i := 1; i < len(values); i++ {
		change := values[i] - values[i-1]
		if math.Abs(change) > stats.StdDev*ta.options.TrendSensitivity {
			changeType := "improvement"
			if change < 0 {
				changeType = "degradation"
			}

			magnitude := math.Abs(change) / stats.StdDev
			impact := "low"
			if magnitude > 2.0 {
				impact = "high"
			} else if magnitude > 1.0 {
				impact = "medium"
			}

			changes = append(changes, SignificantChange{
				Timestamp:   timestamps[i],
				ChangeType:  changeType,
				Magnitude:   magnitude,
				Description: fmt.Sprintf("Significant %s detected", changeType),
				Impact:      impact,
				Confidence:  math.Min(magnitude/3.0, 1.0),
			})
		}
	}

	return changes
}

func (ta *TrendAnalyzer) calculateStatistics(values []float64) TrendStatistics {
	if len(values) == 0 {
		return TrendStatistics{}
	}

	// Sort for median calculation
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate median
	var median float64
	n := len(sorted)
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		median = sorted[n/2]
	}

	// Calculate variance and standard deviation
	variance := 0.0
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(values))
	stdDev := math.Sqrt(variance)

	// Min and max
	min := sorted[0]
	max := sorted[len(sorted)-1]

	// Skewness and kurtosis (simplified calculations)
	skewness := ta.calculateSkewness(values, mean, stdDev)
	kurtosis := ta.calculateKurtosis(values, mean, stdDev)

	return TrendStatistics{
		Mean:     mean,
		Median:   median,
		StdDev:   stdDev,
		Min:      min,
		Max:      max,
		Variance: variance,
		Skewness: skewness,
		Kurtosis: kurtosis,
	}
}

func (ta *TrendAnalyzer) calculateSkewness(values []float64, mean, stdDev float64) float64 {
	if stdDev == 0 || len(values) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, v := range values {
		normalized := (v - mean) / stdDev
		sum += normalized * normalized * normalized
	}

	return sum / float64(len(values))
}

func (ta *TrendAnalyzer) calculateKurtosis(values []float64, mean, stdDev float64) float64 {
	if stdDev == 0 || len(values) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, v := range values {
		normalized := (v - mean) / stdDev
		sum += normalized * normalized * normalized * normalized
	}

	return sum/float64(len(values)) - 3.0 // Excess kurtosis
}

func (ta *TrendAnalyzer) getPreviousValue(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}
	return values[len(values)-2]
}

func (ta *TrendAnalyzer) calculateAbsoluteChange(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}
	return values[len(values)-1] - values[len(values)-2]
}

func (ta *TrendAnalyzer) calculatePercentageChange(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}
	previous := values[len(values)-2]
	current := values[len(values)-1]

	if previous == 0 {
		if current == 0 {
			return 0.0
		}
		return 100.0 // Arbitrarily large change
	}

	return (current - previous) / previous * 100.0
}

func (ta *TrendAnalyzer) calculateEnvironmentTrendStats(data []HistoricalAnalysisResult) EnvironmentTrendStats {
	// Group data by environment attributes
	platformData := make(map[string][]float64)
	cloudData := make(map[string][]float64)
	regionData := make(map[string][]float64)
	versionData := make(map[string][]float64)

	for _, result := range data {
		healthScore := ta.calculateHealthScore(result.Results)

		if result.Environment.Platform != "" {
			platformData[result.Environment.Platform] = append(platformData[result.Environment.Platform], healthScore)
		}
		if result.Environment.CloudProvider != "" {
			cloudData[result.Environment.CloudProvider] = append(cloudData[result.Environment.CloudProvider], healthScore)
		}
		if result.Environment.Region != "" {
			regionData[result.Environment.Region] = append(regionData[result.Environment.Region], healthScore)
		}
		if result.Environment.Version != "" {
			versionData[result.Environment.Version] = append(versionData[result.Environment.Version], healthScore)
		}
	}

	// Calculate trend metrics for each group
	return EnvironmentTrendStats{
		ByPlatform:      ta.calculateGroupTrendMetrics(platformData),
		ByCloudProvider: ta.calculateGroupTrendMetrics(cloudData),
		ByRegion:        ta.calculateGroupTrendMetrics(regionData),
		ByVersion:       ta.calculateGroupTrendMetrics(versionData),
	}
}

func (ta *TrendAnalyzer) calculateGroupTrendMetrics(groupData map[string][]float64) map[string]TrendMetric {
	result := make(map[string]TrendMetric)

	for key, values := range groupData {
		if len(values) < 2 {
			continue
		}

		// Create timestamps (simplified)
		timestamps := make([]time.Time, len(values))
		base := time.Now().Add(-time.Duration(len(values)) * time.Hour)
		for i := range timestamps {
			timestamps[i] = base.Add(time.Duration(i) * time.Hour)
		}

		trend, confidence := ta.detectTrend(values, timestamps)

		result[key] = TrendMetric{
			Current:       values[len(values)-1],
			Previous:      ta.getPreviousValue(values),
			Change:        ta.calculateAbsoluteChange(values),
			ChangePercent: ta.calculatePercentageChange(values),
			Trend:         trend,
			Confidence:    confidence,
			Statistics:    ta.calculateStatistics(values),
		}
	}

	return result
}

func (ta *TrendAnalyzer) calculateDataCompleteness(data []HistoricalAnalysisResult) float64 {
	if len(data) == 0 {
		return 0.0
	}

	// Simple metric: percentage of results with non-empty analysis results
	complete := 0
	for _, result := range data {
		if len(result.Results) > 0 {
			complete++
		}
	}

	return float64(complete) / float64(len(data))
}

func (ta *TrendAnalyzer) calculateDataQuality(data []HistoricalAnalysisResult) float64 {
	if len(data) == 0 {
		return 0.0
	}

	// Simple metric: percentage of successful analyses
	successful := 0
	for _, result := range data {
		if result.Success {
			successful++
		}
	}

	return float64(successful) / float64(len(data))
}

func (ta *TrendAnalyzer) calculateStatisticalPower(dataPoints int) float64 {
	// Simple heuristic for statistical power based on sample size
	if dataPoints < 5 {
		return 0.0
	} else if dataPoints < 10 {
		return 0.3
	} else if dataPoints < 30 {
		return 0.7
	} else {
		return 0.9
	}
}

// Placeholder implementations for remaining methods
func (ta *TrendAnalyzer) generateTrendRecommendations(analyzerTrends []AnalyzerTrend, systemTrends SystemTrend, summary TrendSummary) []TrendRecommendation {
	var recommendations []TrendRecommendation

	// Add recommendations based on trends
	if summary.HealthTrend == TrendDegrading && summary.TrendConfidence > 0.7 {
		recommendations = append(recommendations, TrendRecommendation{
			ID:          "health-degradation",
			Type:        RecommendationInvestigation,
			Priority:    PriorityHigh,
			Title:       "System Health Degradation Detected",
			Description: "Overall system health is trending downward with high confidence.",
			Impact:      "System reliability may be compromised",
			Confidence:  summary.TrendConfidence,
			Actions:     []string{"Review recent changes", "Investigate failing analyzers", "Check resource utilization"},
			Evidence:    []string{fmt.Sprintf("Health trend: %s", summary.HealthTrend), fmt.Sprintf("Confidence: %.2f", summary.TrendConfidence)},
			Timeline:    "Immediate attention required",
		})
	}

	return recommendations
}

func (ta *TrendAnalyzer) performHistoricalComparisons(data []HistoricalAnalysisResult) []HistoricalComparison {
	// Placeholder implementation
	return []HistoricalComparison{}
}

func (ta *TrendAnalyzer) generateInsights(analyzerTrends []AnalyzerTrend, systemTrends SystemTrend, data []HistoricalAnalysisResult) []TrendInsight {
	// Placeholder implementation
	return []TrendInsight{}
}
