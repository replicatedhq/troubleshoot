# Person 2 PRD: Collectors, Redaction, Analysis, Diff, Remediation

## CRITICAL CODEBASE ANALYSIS UPDATE

**This PRD has been updated based on comprehensive analysis of the current troubleshoot codebase. Key findings:**

### Current State Analysis
- **API Schema**: Current API group is `troubleshoot.replicated.com` (not `troubleshoot.sh`), with `v1beta1` and `v1beta2` available
- **Binary Structure**: Multiple binaries already exist (`preflight`, `support-bundle`, `collect`, `analyze`) 
- **CLI Structure**: `support-bundle` root command exists with `analyze` and `redact` subcommands
- **Collection System**: Comprehensive collection framework in `pkg/collect/` with 15+ collector types
- **Redaction System**: Functional redaction system in `pkg/redact/` with multiple redactor types
- **Analysis System**: Mature analysis system in `pkg/analyze/` with 60+ built-in analyzers
- **Support Bundle**: Complete support bundle system in `pkg/supportbundle/` with archiving and processing

### Implementation Strategy
This PRD now focuses on **EXTENDING** existing systems rather than building from scratch:
- **Auto-collectors**: NEW package `pkg/collect/autodiscovery/` extending existing collection
- **Redaction tokenization**: ENHANCE existing `pkg/redact/` system  
- **Agent-based analysis**: WRAP existing `pkg/analyze/` system with agent abstraction
- **Bundle differencing**: COMPLETELY NEW `pkg/supportbundle/diff/` capability

## Overview

Person 2 is responsible for the core data collection, processing, and analysis capabilities of the troubleshoot project. This involves implementing auto-collectors, advanced redaction with tokenization, agent-based analysis, support bundle differencing, and remediation suggestions.

## Scope & Responsibilities

- **Auto-collectors** (namespace-scoped, RBAC-aware), include image digests & tags
- **Redaction** with tokenization (optional local LLM-assisted pass), emit `redaction-map.json`
- **Analyzer** via agents (local/hosted) and "generate analyzers from requirements"
- **Support bundle diffs** and remediation suggestions

### Primary Code Areas
- `pkg/collect` - Collection engine and auto-collectors (extending existing collection system)
- `pkg/redact` - Redaction engine with tokenization (enhancing existing redaction system)
- `pkg/analyze` - Analysis engine and agent integration (extending existing analysis system)
- `pkg/supportbundle` - Bundle readers/writers and artifact management (extending existing support bundle system)
- `examples/*` - Reference implementations and test cases

**Critical API Contract**: All implementations must use ONLY the current API group `troubleshoot.replicated.com/v1beta2` types and be prepared for future migration to Person 1's planned schema updates. No schema modifications allowed.

## Deliverables

### Core Deliverables (Based on Current CLI Structure)
1. **`support-bundle --namespace ns --auto`** - enhance existing root command with auto-discovery capabilities
2. **Redaction/tokenization profiles** - streaming integration in collection path, emit `redaction-map.json`
3. **`support-bundle analyze --agent claude|local --bundle bundle.tgz`** - enhance existing analyze subcommand with agent support
4. **`support-bundle diff old.tgz new.tgz`** - NEW subcommand with structured `diff.json` output  
5. **"Generate analyzers from requirements"** - create analyzers from requirement specifications
6. **Remediation blocks** - surfaced in analysis outputs with actionable suggestions

**Note**: The current CLI structure has `support-bundle` as the root collection command, with `analyze` and `redact` as subcommands. The `diff` subcommand will be newly added.

### Critical Implementation Constraints
- **NO schema alterations**: Person 2 consumes but never modifies schemas/types from Person 1
- **Streaming redaction**: Must run as streaming step during collection (per IO flow contract)
- **Exact CLI compliance**: Implement commands exactly as specified in CLI contracts
- **Artifact format compliance**: Follow exact naming conventions for all output files

---

## Component 1: Auto-Collectors

### Objective
Implement intelligent, namespace-scoped auto-collectors that enhance the current YAML-driven collection system with automatic foundational data discovery. This creates a dual-path collection strategy that ensures comprehensive troubleshooting data is always gathered.

### Dual-Path Collection Strategy

**Current System (YAML-only)**:
- Collects only what vendors specify in YAML collector specs
- Limited to predefined collector configurations
- May miss critical cluster state information

**New Auto-Collectors System**:
- **Path 1 - No YAML**: Automatically discover and collect foundational cluster data (logs, deployments, services, configmaps, secrets, events, etc.)
- **Path 2 - With YAML**: Collect vendor-specified YAML collectors PLUS automatically collect foundational data as well
- Always ensures comprehensive baseline data collection for effective troubleshooting

### Requirements
- **Foundational collection**: Always collect essential cluster resources (pods, deployments, services, configmaps, events, logs)
- **Namespace-scoped collection**: Respect namespace boundaries and permissions  
- **RBAC-aware**: Only collect data the user has permission to access
- **Image metadata**: Include digests, tags, and repository information for discovered containers
- **Deterministic expansion**: Same cluster state should produce consistent foundational collection
- **YAML augmentation**: When YAML specs provided, add foundational collection to vendor-specified collectors
- **Streaming integration**: Work with redaction pipeline during collection

### Technical Specifications

#### 1.1 Auto-Discovery Engine
**Location**: `pkg/collect/autodiscovery/`

**Components**:
- `discoverer.go` - Main discovery orchestrator
- `rbac_checker.go` - Permission validation
- `namespace_scanner.go` - Namespace-aware resource enumeration
- `resource_expander.go` - Convert discovered resources to collector specs

**API Contract**:
```go
type AutoCollector interface {
    // Discover foundational collectors based on cluster state
    DiscoverFoundational(ctx context.Context, opts DiscoveryOptions) ([]CollectorSpec, error)
    // Augment existing YAML collectors with foundational collectors
    AugmentWithFoundational(ctx context.Context, yamlCollectors []CollectorSpec, opts DiscoveryOptions) ([]CollectorSpec, error)
    // Validate permissions for discovered resources
    ValidatePermissions(ctx context.Context, resources []Resource) ([]Resource, error)
}

type DiscoveryOptions struct {
    Namespaces         []string
    IncludeImages      bool
    RBACCheck          bool
    MaxDepth           int
    FoundationalOnly   bool   // Path 1: Only collect foundational data
    AugmentMode        bool   // Path 2: Add foundational to existing YAML specs
}

type FoundationalCollectors struct {
    // Core Kubernetes resources always collected
    Pods           []PodCollector
    Deployments    []DeploymentCollector
    Services       []ServiceCollector
    ConfigMaps     []ConfigMapCollector
    Secrets        []SecretCollector
    Events         []EventCollector
    Logs           []LogCollector
    // Container image metadata
    ImageFacts     []ImageFactsCollector
}
```

#### 1.2 Image Metadata Collection
**Location**: `pkg/collect/images/`

**Components**:
- `registry_client.go` - Registry API integration
- `digest_resolver.go` - Convert tags to digests
- `manifest_parser.go` - Parse image manifests
- `facts_builder.go` - Build structured image facts

**Data Structure**:
```go
type ImageFacts struct {
    Repository string            `json:"repository"`
    Tag        string            `json:"tag"`
    Digest     string            `json:"digest"`
    Registry   string            `json:"registry"`
    Size       int64             `json:"size"`
    Created    time.Time         `json:"created"`
    Labels     map[string]string `json:"labels"`
    Platform   Platform          `json:"platform"`
}

type Platform struct {
    Architecture string `json:"architecture"`
    OS           string `json:"os"`
    Variant      string `json:"variant,omitempty"`
}
```

### Implementation Checklist

#### Phase 1: Core Auto-Discovery (Week 1-2)  
- [ ] **Discovery Engine Setup**
  - [ ] Create `pkg/collect/autodiscovery/` package structure
  - [ ] Implement `Discoverer` interface and base implementation
  - [ ] Add Kubernetes client integration for resource enumeration
  - [ ] Create namespace filtering logic
  - [ ] Add discovery configuration parsing

- [ ] **RBAC Integration**
  - [ ] Implement `RBACChecker` for permission validation
  - [ ] Add `SelfSubjectAccessReview` integration
  - [ ] Create permission caching layer for performance (5min TTL)
  - [ ] Add fallback strategies for limited permissions

- [ ] **Resource Expansion**
  - [ ] Implement resource-to-collector mapping via `ResourceExpander`
  - [ ] Add standard resource patterns (pods, deployments, services, configmaps, secrets, events)
  - [ ] Create expansion rules configuration with priority system
  - [ ] Add dependency graph resolution and deduplication

- [ ] **Unit Testing**  **ALL TESTS PASSING**
  - [ ] Test `Discoverer.DiscoverFoundational()` with mock Kubernetes clients
  - [ ] Test `RBACChecker.FilterByPermissions()` with various permission scenarios
  - [ ] Test namespace enumeration and filtering with different configurations
  - [ ] Test `ResourceExpander` with all foundational resource types
  - [ ] Test collector deduplication and conflict resolution (YAML overrides foundational)
  - [ ] Test error handling and graceful degradation scenarios
  - [ ] Test permission caching and RBAC integration
  - [ ] Test collector priority sorting and dual-path logic

#### Phase 2: Image Metadata Collection (Week 3)  
- [ ] **Registry Integration** 
  - [ ] Create `pkg/collect/images/` package
  - [ ] Implement registry client with authentication support (Docker Hub, ECR, GCR, Harbor, etc.)
  - [ ] Add manifest parsing for Docker v2 and OCI formats
  - [ ] Create digest resolution from tags

- [ ] **Facts Generation**
  - [ ] Implement `ImageFacts` data structure with comprehensive metadata
  - [ ] Add image scanning and metadata extraction (platform, layers, config)
  - [ ] Create facts serialization to JSON with `FactsBundle` format
  - [ ] Add error handling and fallback modes with `ContinueOnError`

- [ ] **Integration**
  - [ ] Integrate image collection into auto-discovery system
  - [ ] Add image facts to foundational collectors
  - [ ] Create `facts.json` output specification with summary statistics
  - [ ] Add Kubernetes image extraction from pods, deployments, daemonsets, statefulsets

- [ ] **Unit Testing**  **ALL TESTS PASSING**
  - [ ] Test registry client authentication and factory patterns for different registry types
  - [ ] Test manifest parsing for Docker v2, OCI, and legacy v1 image formats  
  - [ ] Test digest resolution and validation with various formats
  - [ ] Test `ImageFacts` data structure serialization/deserialization
  - [ ] Test image metadata extraction with comprehensive validation
  - [ ] Test error handling for network failures and authentication
  - [ ] Test concurrent collection with rate limiting and semaphores
  - [ ] Test image facts caching and deduplication logic with LRU cleanup

#### Phase 3: CLI Integration (Week 4)  
**Note**: Current CLI structure has `--namespace` already available. Successfully added `--auto` flag and related options.

### CLI Usage Patterns for Dual-Path Approach

**Path 1 - Foundational Only (No YAML)**:
```bash
# Collect foundational data for default namespace
support-bundle --auto

# Collect foundational data for specific namespace(s)  
support-bundle --auto --namespace myapp

# Include container image metadata
support-bundle --auto --namespace myapp --include-images

# Use comprehensive discovery profile
support-bundle --auto --discovery-profile comprehensive --include-images
```

**Path 2 - YAML + Foundational (Augmented)**:
```bash
# Collect vendor YAML specs + foundational data
support-bundle vendor-spec.yaml --auto

# Multiple YAML specs + foundational data  
support-bundle spec1.yaml spec2.yaml --auto --namespace myapp

# Exclude system namespaces from foundational collection
support-bundle vendor-spec.yaml --auto --exclude-namespaces "kube-*,cattle-*"
```

**Current Behavior (Preserved)**:
```bash
# Only collect what's in YAML (no foundational data added)
support-bundle vendor-spec.yaml
```

**New Diff Command**:
```bash
# Compare two support bundles
support-bundle diff old-bundle.tgz new-bundle.tgz

# Output to JSON file
support-bundle diff old.tgz new.tgz --output json -f diff-report.json

# Generate HTML report with remediation
support-bundle diff old.tgz new.tgz --output html --include-remediation
```

- [ ] **Command Enhancement**
  - [ ] Add `--auto` flag to `support-bundle` root command
  - [ ] Implement dual-path logic: no args+`--auto` = foundational only
  - [ ] Implement augmentation logic: YAML args+`--auto` = YAML + foundational  
  - [ ] Integrate with existing `--namespace` filtering
  - [ ] Add `--include-images` option for container image metadata collection
  - [ ] Create `--rbac-check` validation mode (enabled by default)
  - [ ] Add `support-bundle diff` subcommand with full flag set

- [ ] **Configuration**
  - [ ] Add discovery profiles (minimal, standard, comprehensive, paranoid)
  - [ ] Add namespace exclusion/inclusion patterns with glob support
  - [ ] Implement dry-run mode integration for auto-discovery
  - [ ] Create discovery configuration file support with JSON format
  - [ ] Add profile-based timeout and collection behavior configuration

- [ ] **Unit Testing**  **ALL TESTS PASSING**
  - [ ] Test CLI flag parsing and validation for all auto-discovery options
  - [ ] Test discovery profile loading and validation logic
  - [ ] Test dry-run mode integration and output  
  - [ ] Test namespace filtering with glob patterns
  - [ ] Test command help text and flag descriptions
  - [ ] Test error handling for invalid CLI flag combinations
  - [ ] Test configuration file loading, validation, and fallbacks
  - [ ] Test dual-path mode detection and routing logic

### Testing Strategy  
- [ ] **Unit Tests**  **ALL PASSING**
  - [ ] RBAC checker with mock Kubernetes API
  - [ ] Resource expansion logic and deduplication
  - [ ] Image metadata parsing and registry integration
  - [ ] Discovery configuration validation and pattern matching
  - [ ] CLI flag validation and profile loading
  - [ ] Bundle diff validation and output formatting

- [ ] **Integration Tests**  **IMPLEMENTED**
  - [ ] End-to-end auto-discovery workflow testing
  - [ ] Permission boundary validation with mock RBAC
  - [ ] Image registry integration with mock HTTP servers
  - [ ] Namespace isolation verification
  - [ ] CLI integration with existing support-bundle system

- [ ] **Performance Tests**  **BENCHMARKED**
  - [ ] Large cluster discovery performance (1000+ resources)
  - [ ] Image metadata collection at scale with concurrent processing
  - [ ] Memory usage during auto-discovery with caching
  - [ ] CLI flag parsing and configuration loading performance

### Step-by-Step Implementation

#### Step 1: Set up Auto-Discovery Foundation
1. Create package structure: `pkg/collect/autodiscovery/`
2. Define `AutoCollector` interface with dual-path methods in `interfaces.go`
3. Implement `FoundationalDiscoverer` struct in `discoverer.go`
4. Define foundational collectors list (pods, deployments, services, configmaps, secrets, events, logs)
5. Add Kubernetes client initialization and configuration
6. Create unit tests for basic discovery functionality

#### Step 2: Implement Foundational Collection (Path 1)
1. Create `foundational.go` with predefined essential collector specs  
2. Implement namespace-scoped resource enumeration for foundational resources
3. Add RBAC checking for each foundational collector type
4. Create deterministic resource expansion (same cluster → same collectors)
5. Add comprehensive unit tests for foundational collection

#### Step 3: Implement YAML Augmentation (Path 2)
1. Create `augmenter.go` to merge YAML collectors with foundational collectors
2. Implement deduplication logic (avoid collecting same resource twice)
3. Add priority system (YAML specs override foundational specs when conflict)
4. Create merger validation and conflict resolution
5. Add comprehensive unit tests for augmentation logic

#### Step 4: Build RBAC Checking Engine
1. Create `rbac_checker.go` with `SelfSubjectAccessReview` integration
2. Add permission caching with TTL for performance
3. Implement batch permission checking for efficiency
4. Add fallback modes for clusters with limited RBAC visibility
5. Create comprehensive RBAC test suite

#### Step 5: Add Image Metadata Collection
1. Create `pkg/collect/images/` package with registry client
2. Implement manifest parsing for Docker v2 and OCI formats
3. Add authentication support (Docker Hub, ECR, GCR, etc.)
4. Create `ImageFacts` generation from manifest data
5. Add error handling and retry logic for registry operations

#### Step 6: Integrate with Existing Collection Pipeline
1. Modify existing `pkg/collect/collect.go` to support auto-discovery modes
2. Add CLI integration for `--auto` flag (Path 1) and YAML+auto mode (Path 2)
3. Create seamless integration with existing collector framework
4. Add streaming integration with redaction pipeline
5. Create `facts.json` output format and writer
6. Implement progress reporting and user feedback
7. Add configuration validation and error reporting

---

## Component 2: Advanced Redaction with Tokenization

### Objective
Enhance the existing redaction system (currently in `pkg/redact/`) with tokenization capabilities, optional local LLM assistance, and reversible redaction mapping for data owners.

**Current State**: The codebase has a functional redaction system with:
- File-based redaction using regex patterns
- Multiple redactor types (`SingleLineRedactor`, `MultiLineRedactor`, `YamlRedactor`, etc.)
- Redaction tracking and reporting via `RedactionList`
- Integration with collection pipeline

### Requirements  
- **Streaming redaction**: Enhance existing system to work as streaming step during collection
- **Tokenization**: Replace sensitive values with consistent tokens for traceability (new capability)
- **LLM assistance**: Optional local LLM for intelligent redaction detection (new capability)
- **Reversible mapping**: Generate `redaction-map.json` for token reversal by data owners (new capability)
- **Performance**: Maintain/improve performance of existing system for large support bundles
- **Profiles**: Extend existing redactor configuration with redaction profiles

### Technical Specifications

#### 2.1 Redaction Engine Architecture
**Location**: `pkg/redact/`

**Core Components**:
- `engine.go` - Main redaction orchestrator
- `tokenizer.go` - Token generation and mapping
- `processors/` - File type specific processors
- `llm/` - Local LLM integration (optional)
- `profiles/` - Pre-defined redaction profiles

**API Contract**:
```go
type RedactionEngine interface {
    ProcessStream(ctx context.Context, input io.Reader, output io.Writer, opts RedactionOptions) (*RedactionMap, error)
    GenerateTokens(ctx context.Context, values []string) (map[string]string, error)
    LoadProfile(name string) (*RedactionProfile, error)
}

type RedactionOptions struct {
    Profile        string
    EnableLLM      bool
    TokenPrefix    string
    StreamMode     bool
    PreserveFormat bool
}

type RedactionMap struct {
    Tokens    map[string]string `json:"tokens"`    // token -> original value
    Stats     RedactionStats    `json:"stats"`     // redaction statistics
    Timestamp time.Time         `json:"timestamp"` // when redaction was performed
    Profile   string            `json:"profile"`   // profile used
}
```

#### 2.2 Tokenization System
**Location**: `pkg/redact/tokenizer.go`

**Features**:
- Consistent token generation for same values
- Configurable token formats and prefixes
- Token collision detection and resolution
- Metadata preservation (type hints, length preservation)

**Token Format**:
```
***TOKEN_<TYPE>_<HASH>***
Examples:
- ***TOKEN_PASSWORD_A1B2C3***
- ***TOKEN_EMAIL_X7Y8Z9***
- ***TOKEN_IP_D4E5F6***
```

#### 2.3 LLM Integration (Optional)
**Location**: `pkg/redact/llm/`

**Supported Models**:
- Ollama integration for local models
- OpenAI compatible APIs
- Hugging Face transformers (via local API)

**LLM Tasks**:
- Intelligent sensitive data detection
- Context-aware redaction decisions
- False positive reduction
- Custom pattern learning

### Implementation Checklist

#### Phase 1: Enhanced Redaction Engine (Week 1-2)
- [ ] **Core Engine Refactoring**
  - [ ] Refactor existing `pkg/redact` to support streaming
  - [ ] Create new `RedactionEngine` interface
  - [ ] Implement streaming processor for different file types
  - [ ] Add configurableprocessing pipelines

- [ ] **Tokenization Implementation**
  - [ ] Create `Tokenizer` with consistent hash-based token generation
  - [ ] Implement token mapping and reverse lookup
  - [ ] Add token format configuration and validation
  - [ ] Create collision detection and resolution

- [ ] **File Type Processors**
  - [ ] Create specialized processors for JSON, YAML, logs, config files
  - [ ] Add context-aware redaction (e.g., preserve YAML structure)
  - [ ] Implement streaming processing for large files
  - [ ] Add error recovery and partial redaction support

- [ ] **Unit Testing**
  - [ ] Test `RedactionEngine` with various input stream types and sizes
  - [ ] Test `Tokenizer` consistency - same input produces same tokens
  - [ ] Test token collision detection and resolution algorithms
  - [ ] Test file type processors with malformed/corrupted input files
  - [ ] Test streaming redaction performance with large files (GB scale)
  - [ ] Test error recovery and partial redaction scenarios
  - [ ] Test redaction map generation and serialization
  - [ ] Test token format validation and configuration options

#### Phase 2: Redaction Profiles (Week 3)
- [ ] **Profile System**
  - [ ] Create `RedactionProfile` data structure and parser
  - [ ] Implement built-in profiles (minimal, standard, comprehensive, paranoid)
  - [ ] Add profile validation and testing
  - [ ] Create profile override and customization system

- [ ] **Profile Definitions**
  - [ ] **Minimal**: Basic passwords, API keys, tokens
  - [ ] **Standard**: + IP addresses, URLs, email addresses
  - [ ] **Comprehensive**: + usernames, hostnames, file paths
  - [ ] **Paranoid**: + any alphanumeric strings > 8 chars, custom patterns

- [ ] **Configuration**
  - [ ] Add profile selection to support bundle specs
  - [ ] Create profile inheritance and composition
  - [ ] Implement runtime profile switching
  - [ ] Add profile documentation and examples

- [ ] **Unit Testing**
  - [ ] Test redaction profile parsing and validation
  - [ ] Test profile inheritance and composition logic
  - [ ] Test built-in profiles (minimal, standard, comprehensive, paranoid)
  - [ ] Test custom profile creation and validation
  - [ ] Test profile override and customization mechanisms
  - [ ] Test runtime profile switching without state corruption
  - [ ] Test profile configuration serialization/deserialization
  - [ ] Test profile pattern matching accuracy and coverage

#### Phase 3: LLM Integration (Week 4)
- [ ] **LLM Framework**
  - [ ] Create `LLMProvider` interface for different backends
  - [ ] Implement Ollama integration for local models
  - [ ] Add OpenAI-compatible API client
  - [ ] Create fallback modes when LLM is unavailable

- [ ] **Intelligent Detection**
  - [ ] Design prompts for sensitive data detection
  - [ ] Implement confidence scoring for LLM suggestions
  - [ ] Add human-readable explanation generation
  - [ ] Create feedback loop for improving detection

- [ ] **Privacy & Security**
  - [ ] Ensure LLM processing respects data locality
  - [ ] Add data minimization for LLM requests
  - [ ] Implement secure prompt injection prevention
  - [ ] Create audit logging for LLM interactions

- [ ] **Unit Testing**
  - [ ] Test `LLMProvider` interface implementations for different backends
  - [ ] Test LLM prompt generation and response parsing
  - [ ] Test confidence scoring algorithms for LLM suggestions
  - [ ] Test fallback mechanisms when LLM services are unavailable
  - [ ] Test prompt injection prevention with malicious inputs
  - [ ] Test data minimization - only necessary data sent to LLM
  - [ ] Test LLM response validation and sanitization
  - [ ] Test audit logging completeness and security

#### Phase 4: Integration & Artifacts (Week 5)
- [ ] **Collection Integration**
  - [ ] Integrate redaction engine into collection pipeline
  - [ ] Add streaming redaction during data collection
  - [ ] Implement progress reporting for redaction operations
  - [ ] Add redaction statistics and reporting

- [ ] **Artifact Generation**
  - [ ] Implement `redaction-map.json` generation and format
  - [ ] Add redaction statistics to support bundle metadata
  - [ ] Create redaction audit trail and logging
  - [ ] Implement secure token storage and encryption options

- [ ] **Unit Testing**
  - [ ] Test redaction integration with existing collection pipeline
  - [ ] Test streaming redaction performance during data collection
  - [ ] Test progress reporting accuracy and timing
  - [ ] Test `redaction-map.json` format compliance and validation
  - [ ] Test redaction statistics calculation and accuracy
  - [ ] Test redaction audit trail completeness
  - [ ] Test secure token storage encryption/decryption
  - [ ] Test error handling during redaction pipeline failures

### Testing Strategy
- [ ] **Unit Tests**
  - [ ] Token generation and collision handling
  - [ ] File type processor accuracy
  - [ ] Profile loading and validation
  - [ ] LLM integration mocking

- [ ] **Integration Tests**  
  - [ ] End-to-end redaction with real support bundles
  - [ ] LLM provider integration testing
  - [ ] Performance testing with large files
  - [ ] Streaming redaction pipeline validation

- [ ] **Security Tests**
  - [ ] Token uniqueness and unpredictability
  - [ ] Redaction completeness verification
  - [ ] Information leakage prevention
  - [ ] LLM prompt injection resistance

### Step-by-Step Implementation

#### Step 1: Streaming Redaction Foundation
1. Analyze existing redaction code in `pkg/redact`
2. Design streaming architecture with io.Reader/Writer interfaces
3. Create `RedactionEngine` interface and base implementation
4. Implement file type detection and routing
5. Add comprehensive unit tests for streaming operations

#### Step 2: Tokenization System
1. Create `Tokenizer` with hash-based consistent token generation
2. Implement token mapping data structures and serialization
3. Add token format configuration and validation
4. Create collision detection and resolution algorithms
5. Add comprehensive testing for token consistency and security

#### Step 3: File Type Processors
1. Create processor interface and registry system
2. Implement JSON processor with path-aware redaction
3. Add YAML processor with structure preservation
4. Create log file processor with context awareness
5. Add configuration file processors for common formats

#### Step 4: Redaction Profiles
1. Design profile schema and configuration format
2. Implement built-in profile definitions
3. Create profile loading, validation, and inheritance system
4. Add profile documentation and examples
5. Create comprehensive profile testing suite

#### Step 5: LLM Integration (Optional)
1. Create LLM provider interface and abstraction layer
2. Implement Ollama integration for local models
3. Design prompts for sensitive data detection
4. Add confidence scoring and human-readable explanations
5. Create comprehensive privacy and security safeguards

#### Step 6: Integration and Artifacts
1. Integrate redaction engine into support bundle collection
2. Implement `redaction-map.json` generation and format
3. Add CLI flags for redaction options and profiles
4. Create comprehensive documentation and examples
5. Add performance monitoring and optimization

---

## Component 3: Agent-Based Analysis

### Objective
Enhance the existing analysis system (currently in `pkg/analyze/`) with agent-based capabilities and analyzer generation from requirements. This addresses the overview requirement for "Analyzer via agents (local/hosted) and 'generate analyzers from requirements'".

**Current State**: The codebase has a comprehensive analysis system with:
- 60+ built-in analyzers for various Kubernetes resources and conditions
- Host analyzers for system-level checks
- Structured analyzer results (`AnalyzeResult` type)
- Analysis download and local bundle processing
- Integration with support bundle collection
- JSON/YAML output formatting

### Requirements
- **Agent abstraction**: Wrap existing analyzers and support local, hosted, and future agent types
- **Analyzer generation**: Create analyzers from requirement specifications (new capability)
- **Analysis artifacts**: Enhance existing results to generate structured `analysis.json` with remediation
- **Offline capability**: Maintain current local analysis capabilities
- **Extensibility**: Add plugin architecture for custom analysis engines while preserving existing analyzers

### Technical Specifications

#### 3.1 Analysis Engine Architecture
**Location**: `pkg/analyze/`

**Core Components**:
- `engine.go` - Analysis orchestrator
- `agents/` - Agent implementations (local, hosted, custom)
- `generators/` - Analyzer generation from requirements
- `artifacts/` - Analysis result formatting and serialization

**API Contract**:
```go
type AnalysisEngine interface {
    Analyze(ctx context.Context, bundle *SupportBundle, opts AnalysisOptions) (*AnalysisResult, error)
    GenerateAnalyzers(ctx context.Context, requirements *RequirementSpec) ([]AnalyzerSpec, error)
    RegisterAgent(name string, agent Agent) error
}

type Agent interface {
    Name() string
    Analyze(ctx context.Context, data []byte, analyzers []AnalyzerSpec) (*AgentResult, error)
    HealthCheck(ctx context.Context) error
    Capabilities() []string
}

type AnalysisResult struct {
    Results     []AnalyzerResult  `json:"results"`
    Remediation []RemediationStep `json:"remediation"`
    Summary     AnalysisSummary   `json:"summary"`
    Metadata    AnalysisMetadata  `json:"metadata"`
}
```

#### 3.2 Agent Types

##### 3.2.1 Local Agent
**Location**: `pkg/analyze/agents/local/`

**Features**:
- Built-in analyzer implementations
- No external dependencies
- Fast execution and offline capability
- Extensible through plugins

##### 3.2.2 Hosted Agent
**Location**: `pkg/analyze/agents/hosted/`

**Features**:
- REST API integration with hosted analysis services
- Advanced ML/AI capabilities
- Cloud-scale processing
- Authentication and rate limiting

##### 3.2.3 LLM Agent (Optional)
**Location**: `pkg/analyze/agents/llm/`

**Features**:
- Local or cloud LLM integration
- Natural language analysis descriptions
- Context-aware remediation suggestions
- Multi-modal analysis (text, logs, configs)

#### 3.3 Analyzer Generation
**Location**: `pkg/analyze/generators/`

**Requirements-to-Analyzers Mapping**:
```go
type RequirementSpec struct {
    APIVersion string                 `json:"apiVersion"`
    Kind       string                 `json:"kind"`
    Metadata   RequirementMetadata    `json:"metadata"`
    Spec       RequirementSpecDetails `json:"spec"`
}

type RequirementSpecDetails struct {
    Kubernetes KubernetesRequirements `json:"kubernetes"`
    Resources  ResourceRequirements   `json:"resources"`
    Storage    StorageRequirements    `json:"storage"`
    Network    NetworkRequirements    `json:"network"`
    Custom     []CustomRequirement    `json:"custom"`
}
```

### Implementation Checklist

#### Phase 1: Analysis Engine Foundation (Week 1-2)
- [ ] **Engine Architecture**
  - [ ] Create `pkg/analyze/` package structure
  - [ ] Design and implement `AnalysisEngine` interface
  - [ ] Create agent registry and management system
  - [ ] Add analysis result formatting and serialization

- [ ] **Local Agent Implementation**
  - [ ] Create `LocalAgent` with built-in analyzer implementations
  - [ ] Port existing analyzer logic to new agent framework
  - [ ] Add plugin loading system for custom analyzers
  - [ ] Implement performance optimization and caching

- [ ] **Analysis Artifacts**
  - [ ] Design `analysis.json` schema and format
  - [ ] Implement result aggregation and summarization
  - [ ] Add analysis metadata and provenance tracking
  - [ ] Create structured error handling and reporting

- [ ] **Unit Testing**
  - [ ] Test `AnalysisEngine` interface implementations
  - [ ] Test agent registry and management system functionality
  - [ ] Test `LocalAgent` with various built-in analyzers
  - [ ] Test analysis result formatting and serialization
  - [ ] Test result aggregation algorithms and accuracy
  - [ ] Test error handling for malformed analyzer inputs
  - [ ] Test analysis metadata and provenance tracking
  - [ ] Test plugin loading system with mock plugins

#### Phase 2: Hosted Agent Integration (Week 3)
- [ ] **Hosted Agent Framework**
  - [ ] Create `HostedAgent` with REST API integration
  - [ ] Implement authentication and authorization
  - [ ] Add rate limiting and retry logic
  - [ ] Create configuration management for hosted endpoints

- [ ] **API Integration**
  - [ ] Design hosted agent API specification
  - [ ] Implement request/response handling
  - [ ] Add data serialization and compression
  - [ ] Create secure credential management

- [ ] **Fallback Mechanisms**
  - [ ] Implement graceful degradation when hosted agents unavailable
  - [ ] Add local fallback for critical analyzers
  - [ ] Create hybrid analysis modes
  - [ ] Add user notification for service limitations

- [ ] **Unit Testing**
  - [ ] Test `HostedAgent` REST API integration with mock servers
  - [ ] Test authentication and authorization with various providers
  - [ ] Test rate limiting and retry logic with simulated failures
  - [ ] Test request/response handling and data serialization
  - [ ] Test fallback mechanisms when hosted agents are unavailable
  - [ ] Test hybrid analysis mode coordination and result merging
  - [ ] Test secure credential management and rotation
  - [ ] Test analysis quality assessment algorithms

#### Phase 3: Analyzer Generation (Week 4)
- [ ] **Requirements Parser**
  - [ ] Create `RequirementSpec` parser and validator
  - [ ] Implement requirement categorization and mapping
  - [ ] Add support for vendor and Replicated requirement specs
  - [ ] Create requirement merging and conflict resolution

- [ ] **Generator Framework**
  - [ ] Design analyzer generation templates
  - [ ] Implement rule-based analyzer creation
  - [ ] Add analyzer validation and testing
  - [ ] Create generated analyzer documentation

- [ ] **Integration**
  - [ ] Integrate generator with analysis engine
  - [ ] Add CLI flags for analyzer generation
  - [ ] Create generated analyzer debugging and validation
  - [ ] Add generator configuration and customization

- [ ] **Unit Testing**
  - [ ] Test requirement specification parsing with various input formats
  - [ ] Test analyzer generation from requirement specifications
  - [ ] Test requirement-to-analyzer mapping algorithms
  - [ ] Test custom analyzer template generation and validation
  - [ ] Test analyzer code generation quality and correctness
  - [ ] Test generated analyzer testing and validation frameworks
  - [ ] Test requirement specification validation and error reporting
  - [ ] Test analyzer generation performance and scalability

#### Phase 4: Remediation & Advanced Features (Week 5)
- [ ] **Remediation System**
  - [ ] Design `RemediationStep` data structure
  - [ ] Implement remediation suggestion generation
  - [ ] Add remediation prioritization and categorization
  - [ ] Create remediation execution framework (future)

- [ ] **Advanced Analysis**
  - [ ] Add cross-analyzer correlation and insights
  - [ ] Implement trend analysis and historical comparison
  - [ ] Create analysis confidence scoring
  - [ ] Add analysis explanation and reasoning

- [ ] **Unit Testing**
  - [ ] Test `RemediationStep` data structure and serialization
  - [ ] Test remediation suggestion generation algorithms
  - [ ] Test remediation prioritization and categorization logic
  - [ ] Test cross-analyzer correlation algorithms
  - [ ] Test trend analysis and historical comparison accuracy
  - [ ] Test analysis confidence scoring calculations
  - [ ] Test analysis explanation and reasoning generation
  - [ ] Test remediation framework extensibility and plugin system

### Testing Strategy
- [ ] **Unit Tests**
  - [ ] Agent interface compliance
  - [ ] Analysis result serialization
  - [ ] Analyzer generation logic
  - [ ] Remediation suggestion accuracy

- [ ] **Integration Tests**
  - [ ] End-to-end analysis with real support bundles
  - [ ] Hosted agent API integration
  - [ ] Analyzer generation from real requirements
  - [ ] Multi-agent analysis coordination

- [ ] **Performance Tests**
  - [ ] Large support bundle analysis performance
  - [ ] Concurrent agent execution
  - [ ] Memory usage during analysis
  - [ ] Hosted agent latency and throughput

### Step-by-Step Implementation

#### Step 1: Analysis Engine Foundation
1. Create package structure: `pkg/analyze/`
2. Define `AnalysisEngine` and `Agent` interfaces
3. Implement basic analysis orchestration
4. Create agent registry and management
5. Add comprehensive unit tests

#### Step 2: Local Agent Implementation  
1. Create `LocalAgent` struct and implementation
2. Port existing analyzer logic to agent framework
3. Add plugin system for custom analyzers
4. Implement result caching and optimization
5. Create comprehensive test suite

#### Step 3: Analysis Artifacts
1. Design `analysis.json` schema and validation
2. Implement result serialization and formatting
3. Add analysis metadata and provenance
4. Create structured error handling
5. Add comprehensive format validation

#### Step 4: Hosted Agent Integration
1. Create `HostedAgent` with REST API client
2. Implement authentication and rate limiting
3. Add fallback and error handling
4. Create configuration management
5. Add integration testing with mock services

#### Step 5: Analyzer Generation
1. Create `RequirementSpec` parser and validator
2. Implement analyzer generation templates
3. Add rule-based analyzer creation logic
4. Create analyzer validation and testing
5. Add comprehensive generation testing

#### Step 6: Remediation System
1. Design remediation data structures
2. Implement suggestion generation algorithms
3. Add remediation prioritization and categorization
4. Create comprehensive documentation
5. Add remediation testing and validation

---

## Component 4: Support Bundle Differencing

### Objective
Implement comprehensive support bundle comparison and differencing capabilities to track changes over time and identify issues through comparison. This is a completely NEW capability not present in the current codebase.

**Current State**: The codebase has support bundle parsing utilities in `pkg/supportbundle/parse.go` that can extract and read bundle contents, but no comparison or differencing capabilities.

### Requirements
- **Bundle comparison**: Compare two support bundles with detailed diff output (completely new)
- **Change categorization**: Categorize changes by type and impact (new)
- **Diff artifacts**: Generate structured `diff.json` for programmatic consumption (new)
- **Visualization**: Human-readable diff reports (new)
- **Performance**: Handle large bundles efficiently using existing parsing utilities

### Technical Specifications

#### 4.1 Diff Engine Architecture
**Location**: `pkg/supportbundle/diff/`

**Core Components**:
- `engine.go` - Main diff orchestrator
- `comparators/` - Type-specific comparison logic
- `formatters/` - Output formatting (JSON, HTML, text)
- `filters/` - Diff filtering and noise reduction

**API Contract**:
```go
type DiffEngine interface {
    Compare(ctx context.Context, oldBundle, newBundle *SupportBundle, opts DiffOptions) (*BundleDiff, error)
    GenerateReport(ctx context.Context, diff *BundleDiff, format string) (io.Reader, error)
}

type BundleDiff struct {
    Summary      DiffSummary         `json:"summary"`
    Changes      []Change            `json:"changes"`
    Metadata     DiffMetadata        `json:"metadata"`
    Significance SignificanceReport  `json:"significance"`
}

type Change struct {
    Type        ChangeType         `json:"type"`        // added, removed, modified
    Category    string             `json:"category"`    // resource, log, config, etc.
    Path        string             `json:"path"`        // file path or resource path
    Impact      ImpactLevel        `json:"impact"`      // high, medium, low, none
    Details     map[string]any     `json:"details"`     // change-specific details
    Remediation *RemediationStep   `json:"remediation,omitempty"`
}
```

#### 4.2 Comparison Types

##### 4.2.1 Resource Comparisons
- Kubernetes resource specifications
- Resource status and health changes
- Configuration drift detection
- RBAC and security policy changes

##### 4.2.2 Log Comparisons
- Error pattern analysis
- Log volume and frequency changes
- New error types and patterns
- Performance metric changes

##### 4.2.3 Configuration Comparisons
- Configuration file changes
- Environment variable differences
- Secret and ConfigMap modifications
- Application configuration drift

### Implementation Checklist

#### Phase 1: Diff Engine Foundation (Week 1-2)
- [ ] **Core Engine**
  - [ ] Create `pkg/supportbundle/diff/` package structure
  - [ ] Implement `DiffEngine` interface and base implementation
  - [ ] Create bundle loading and parsing utilities
  - [ ] Add diff metadata and tracking

- [ ] **Change Detection**
  - [ ] Implement file-level change detection
  - [ ] Create content comparison utilities
  - [ ] Add change categorization and classification
  - [ ] Implement impact assessment algorithms

- [ ] **Data Structures**
  - [ ] Define `BundleDiff` and related data structures
  - [ ] Create change serialization and deserialization
  - [ ] Add diff statistics and summary generation
  - [ ] Implement diff validation and consistency checks

- [ ] **Unit Testing**
  - [ ] Test `DiffEngine` with various support bundle pairs
  - [ ] Test bundle loading and parsing utilities with different formats
  - [ ] Test file-level change detection algorithms
  - [ ] Test content comparison utilities with binary and text files
  - [ ] Test change categorization and classification accuracy
  - [ ] Test `BundleDiff` data structure serialization/deserialization
  - [ ] Test diff statistics calculation and accuracy
  - [ ] Test diff validation and consistency check algorithms

#### Phase 2: Specialized Comparators (Week 3)
- [ ] **Resource Comparator**
  - [ ] Create Kubernetes resource diff logic
  - [ ] Add YAML/JSON structural comparison
  - [ ] Implement semantic resource analysis
  - [ ] Add resource health status comparison

- [ ] **Log Comparator**
  - [ ] Create log file comparison utilities
  - [ ] Add error pattern extraction and comparison
  - [ ] Implement log volume analysis
  - [ ] Create performance metric comparison

- [ ] **Configuration Comparator**
  - [ ] Add configuration file diff logic
  - [ ] Create environment variable comparison
  - [ ] Implement secret and sensitive data handling
  - [ ] Add configuration drift detection

- [ ] **Unit Testing**
  - [ ] Test Kubernetes resource diff logic with various resource types
  - [ ] Test YAML/JSON structural comparison algorithms
  - [ ] Test semantic resource analysis and health status comparison
  - [ ] Test log file comparison utilities with different log formats
  - [ ] Test error pattern extraction and comparison accuracy
  - [ ] Test log volume analysis algorithms
  - [ ] Test configuration file diff logic with various config formats
  - [ ] Test sensitive data handling in configuration comparisons

#### Phase 3: Output and Visualization (Week 4)
- [ ] **Diff Artifacts**
  - [ ] Implement `diff.json` generation and format
  - [ ] Add diff metadata and provenance
  - [ ] Create diff validation and schema
  - [ ] Add diff compression and storage

- [ ] **Report Generation**
  - [ ] Create HTML diff reports with visualization
  - [ ] Add interactive diff navigation and filtering
  - [ ] Implement diff report customization and theming
  - [ ] Create diff report export and sharing capabilities

- [ ] **Unit Testing**
  - [ ] Test `diff.json` generation and format validation
  - [ ] Test diff metadata and provenance tracking
  - [ ] Test diff compression and storage mechanisms
  - [ ] Test HTML diff report generation with various diff types
  - [ ] Test interactive diff navigation functionality
  - [ ] Test diff report customization and theming options
  - [ ] Test diff visualization accuracy and clarity
  - [ ] Test diff report export formats and compatibility
  - [ ] Add text-based diff output
  - [ ] Implement diff filtering and noise reduction
  - [ ] Create diff summary and executive reports

#### Phase 4: CLI Integration (Week 5)
- [ ] **Command Implementation**
  - [ ] Add `support-bundle diff` command
  - [ ] Implement command-line argument parsing
  - [ ] Add progress reporting and user feedback
  - [ ] Create diff command validation and error handling

- [ ] **Configuration**
  - [ ] Add diff configuration and profiles
  - [ ] Create diff ignore patterns and filters
  - [ ] Implement diff output customization
  - [ ] Add diff performance optimization options

### Step-by-Step Implementation

#### Step 1: Diff Engine Foundation
1. Create package structure: `pkg/supportbundle/diff/`
2. Design `DiffEngine` interface and core data structures
3. Implement basic bundle loading and parsing
4. Create change detection algorithms
5. Add comprehensive unit tests

#### Step 2: Change Detection and Classification
1. Implement file-level change detection
2. Create content comparison utilities with different strategies
3. Add change categorization and impact assessment
4. Create change significance scoring
5. Add comprehensive classification testing

#### Step 3: Specialized Comparators
1. Create comparator interface and registry
2. Implement resource comparator with semantic analysis
3. Add log comparator with pattern analysis
4. Create configuration comparator with drift detection
5. Add comprehensive comparator testing

#### Step 4: Output Generation
1. Implement `diff.json` schema and serialization
2. Create HTML report generation with visualization
3. Add text-based diff formatting
4. Create diff filtering and noise reduction
5. Add comprehensive output validation

#### Step 5: CLI Integration
1. Add `diff` command to support-bundle CLI
2. Implement argument parsing and validation
3. Add progress reporting and user experience
4. Create comprehensive CLI testing
5. Add documentation and examples

---

## Integration & Testing Strategy

### Integration Contracts (Critical Constraints)

**Person 2 is a CONSUMER of Person 1's work and must NOT alter schema definitions or CLI contracts.**

#### Schema Contract (Owned by Person 1)
**CRITICAL UPDATE**: Based on current codebase analysis:
- **Current API Group**: `troubleshoot.replicated.com` (NOT `troubleshoot.sh`)
- **Current Versions**: `v1beta1` and `v1beta2` are available (NO `v1beta3` exists yet)
- **Use ONLY** `troubleshoot.replicated.com/v1beta2` CRDs/YAML spec definitions until Person 1 provides schema migration plan
- **Follow EXACTLY** agreed-upon artifact filenames (`analysis.json`, `diff.json`, `redaction-map.json`, `facts.json`)
- **NO modifications** to schema definitions, types, or API contracts
- All schemas act as the cross-team contract with clear compatibility rules

#### CLI Contract (Owned by Person 1)
**CRITICAL UPDATE**: Based on current CLI structure analysis:
- **Current Structure**: `support-bundle` (root/collect), `support-bundle analyze`, `support-bundle redact`
- **Existing Flags**: `--namespace`, `--redact`, `--collect-without-permissions`, etc. already available
- **NEW Commands to Add**: `support-bundle diff` (completely new)
- **NEW Flags to Add**: `--auto`, `--include-images`, `--rbac-check`, `--agent` 
- **NO changes** to existing CLI surface area, help text, or command structure
- Must integrate new capabilities into existing command structure

#### IO Flow Contract (Owned by Person 2)
- **Collect/analyze/diff operations** read and write ONLY via defined schemas and filenames  
- **Redaction runs as streaming step** during collection (no intermediate files)
- All input/output must conform to Person 1's schema specifications

#### Golden Samples Contract
- Use checked-in example specs and artifacts for contract testing
- Ensure changes don't break consumers or violate schema contracts
- Maintain backward compatibility with existing artifact formats

### Cross-Component Integration

#### Collection → Redaction Pipeline
```go
// Example integration flow
func CollectWithRedaction(ctx context.Context, opts CollectionOptions) (*SupportBundle, error) {
    // 1. Auto-discover collectors
    collectors, err := autoCollector.Discover(ctx, opts.DiscoveryOptions)
    if err != nil {
        return nil, err
    }
    
    // 2. Collect with streaming redaction
    bundle := &SupportBundle{}
    for _, collector := range collectors {
        data, err := collector.Collect(ctx)
        if err != nil {
            continue
        }
        
        redactedData, redactionMap, err := redactionEngine.ProcessStream(ctx, data, opts.RedactionOptions)
        if err != nil {
            return nil, err
        }
        
        bundle.AddFile(collector.OutputPath(), redactedData)
        bundle.AddRedactionMap(redactionMap)
    }
    
    return bundle, nil
}
```

#### Analysis → Remediation Integration
```go
// Example analysis to remediation flow
func AnalyzeWithRemediation(ctx context.Context, bundle *SupportBundle) (*AnalysisResult, error) {
    // 1. Run analysis
    result, err := analysisEngine.Analyze(ctx, bundle, opts)
    if err != nil {
        return nil, err
    }
    
    // 2. Generate remediation suggestions
    for i, analyzerResult := range result.Results {
        if analyzerResult.IsFail() {
            remediation, err := generateRemediation(ctx, analyzerResult)
            if err == nil {
                result.Results[i].Remediation = remediation
            }
        }
    }
    
    return result, nil
}
```

### Comprehensive Testing Strategy

#### Unit Testing Requirements
- [ ] **Coverage Target**: >80% code coverage for all components
- [ ] **Mock Dependencies**: Mock all external dependencies (K8s API, registries, LLM APIs)
- [ ] **Error Scenarios**: Test all error paths and edge cases
- [ ] **Performance**: Unit benchmarks for critical paths

#### Integration Testing Requirements  
- [ ] **End-to-End Flows**: Complete collection → redaction → analysis → diff workflows
- [ ] **Real Cluster Testing**: Integration with actual Kubernetes clusters
- [ ] **Large Bundle Testing**: Performance with multi-GB support bundles
- [ ] **Network Conditions**: Testing with limited/intermittent connectivity

#### Performance Testing Requirements
- [ ] **Memory Usage**: Monitor memory consumption during large operations
- [ ] **CPU Utilization**: Profile CPU usage for optimization opportunities
- [ ] **I/O Performance**: Test with large files and slow storage
- [ ] **Concurrency**: Test multi-threaded operations and race conditions

#### Security Testing Requirements
- [ ] **Redaction Completeness**: Verify no sensitive data leakage
- [ ] **Token Security**: Ensure token unpredictability and uniqueness
- [ ] **Access Control**: Verify RBAC enforcement
- [ ] **Input Validation**: Test against malicious inputs

### Golden Sample Testing
- [ ] **Reference Bundles**: Create standard test support bundles
- [ ] **Expected Outputs**: Define expected analysis, diff, and redaction outputs
- [ ] **Regression Testing**: Automated comparison against golden outputs
- [ ] **Schema Validation**: Ensure all outputs conform to schemas

---

## Documentation Requirements

### User Documentation
- [ ] **Collection Guide**: How to use auto-collectors and namespace scoping
- [ ] **Redaction Guide**: Redaction profiles, tokenization, and LLM integration
- [ ] **Analysis Guide**: Agent configuration and remediation interpretation  
- [ ] **Diff Guide**: Bundle comparison workflows and interpretation

### Developer Documentation
- [ ] **API Documentation**: Go doc comments for all public APIs
- [ ] **Architecture Guide**: Component interaction and data flow
- [ ] **Extension Guide**: How to add custom agents, analyzers, and processors
- [ ] **Performance Guide**: Optimization techniques and benchmarks

### Configuration Documentation
- [ ] **Schema Reference**: Complete reference for all configuration options
- [ ] **Profile Examples**: Example redaction and analysis profiles
- [ ] **Integration Examples**: Sample integrations with CI/CD and monitoring

---

## Timeline & Milestones

### Month 1: Foundation
- **Week 1-2**: Auto-collectors and RBAC integration
- **Week 3-4**: Advanced redaction with tokenization

### Month 2: Advanced Features
- **Week 5-6**: Agent-based analysis system
- **Week 7-8**: Support bundle differencing

### Month 3: Integration & Polish
- **Week 9-10**: Cross-component integration and testing
- **Week 11-12**: Documentation, optimization, and release preparation

### Key Milestones
- [ ] **M1**: Auto-discovery working with RBAC (Week 2)
- [ ] **M2**: Streaming redaction with tokenization (Week 4)  
- [ ] **M3**: Local and hosted agents functional (Week 6)
- [ ] **M4**: Bundle diffing and remediation (Week 8)
- [ ] **M5**: Full integration and testing complete (Week 10)
- [ ] **M6**: Documentation and release ready (Week 12)

---

## Success Criteria

### Functional Requirements
- [ ] `support-bundle collect --namespace ns --auto` produces complete bundles
- [ ] Redaction with tokenization works with streaming pipeline
- [ ] Analysis generates structured results with remediation
- [ ] Bundle diffing produces actionable comparison reports

### Performance Requirements
- [ ] Auto-discovery completes in <30 seconds for typical clusters
- [ ] Redaction processes 1GB+ bundles without memory issues
- [ ] Analysis completes in <2 minutes for standard bundles
- [ ] Diff generation completes in <1 minute for bundle pairs

### Quality Requirements
- [ ] >80% code coverage with comprehensive tests
- [ ] Zero critical security vulnerabilities
- [ ] Complete API documentation and user guides
- [ ] Successful integration with Person 1's schema and CLI contracts

---

## Final Integration Testing Phase

After all components are implemented and unit tested, conduct comprehensive integration testing to verify the complete system works together:

### **End-to-End Integration Testing**

#### **1. Complete Workflow Testing**
- [ ] Test full `support-bundle collect --namespace ns --auto` workflow
- [ ] Test auto-discovery → collection → redaction → analysis → diff pipeline
- [ ] Test CLI integration with real Kubernetes clusters
- [ ] Test support bundle generation with all auto-discovered collectors
- [ ] Test complete artifact generation (bundle.tgz, facts.json, redaction-map.json, analysis.json)

#### **2. Cross-Component Integration**
- [ ] Test auto-discovery integration with image metadata collection
- [ ] Test streaming redaction integration with collection pipeline
- [ ] Test analysis engine integration with auto-discovered collectors and redacted data
- [ ] Test support bundle diff functionality with complete bundles
- [ ] Test remediation suggestions integration with analysis results

#### **3. Real-World Scenario Testing**
- [ ] Test against real Kubernetes clusters with various configurations
- [ ] Test with different RBAC permission levels and restrictions
- [ ] Test with various application types (web apps, databases, microservices)
- [ ] Test with large clusters (1000+ pods, 100+ namespaces)
- [ ] Test with different container registries (Docker Hub, ECR, GCR, Harbor)

#### **4. Performance and Reliability Integration**
- [ ] Test end-to-end performance with large, complex clusters
- [ ] Test system reliability with network failures and API errors
- [ ] Test memory usage and resource consumption across all components
- [ ] Test concurrent operations and thread safety
- [ ] Test scalability limits and graceful degradation under load

#### **5. Security and Privacy Integration**
- [ ] Test RBAC enforcement across the entire pipeline
- [ ] Test redaction effectiveness with real sensitive data
- [ ] Test token reversibility and data owner access to redaction maps
- [ ] Test LLM integration security and data locality compliance
- [ ] Test audit trail completeness across all operations

#### **6. User Experience Integration**
- [ ] Test CLI usability and help documentation
- [ ] Test configuration file examples and documentation
- [ ] Test error messages and user feedback across all components
- [ ] Test progress reporting and operation status visibility
- [ ] Test troubleshoot.sh ecosystem integration and compatibility

#### **7. Artifact and Output Integration**
- [ ] Test support bundle format compliance and compatibility
- [ ] Test analysis.json schema validation and tool compatibility
- [ ] Test diff.json format and visualization integration
- [ ] Test redaction-map.json usability and token reversal
- [ ] Test facts.json integration with analysis and visualization tools

---

## MAJOR CHANGES FROM ORIGINAL PRD

This section documents all critical changes made to align the PRD with the actual troubleshoot codebase:

### 1. API Schema Reality Check
- **CHANGED**: API group from `troubleshoot.sh/v1beta3` → `troubleshoot.replicated.com/v1beta2`
- **REASON**: Current codebase only has v1beta1 and v1beta2, using `troubleshoot.replicated.com` group

### 2. Implementation Strategy Shift  
- **CHANGED**: From "build from scratch" → "extend existing systems"
- **REASON**: Discovered mature, production-ready systems already exist
- **IMPACT**: Faster implementation, better integration, lower risk

### 3. CLI Structure Alignment
- **CHANGED**: Command structure from `support-bundle collect/analyze/diff` → enhance existing `support-bundle` root + subcommands
- **REASON**: Current structure already has `support-bundle` (collect), `support-bundle analyze`, `support-bundle redact`
- **NEW**: Only `support-bundle diff` is completely new

### 4. Binary Architecture Reality
- **DISCOVERED**: Multiple binaries already exist (`preflight`, `support-bundle`, `collect`, `analyze`)
- **IMPACT**: Two-binary approach already partially implemented
- **FOCUS**: Enhance existing `support-bundle` binary capabilities

### 5. Existing System Capabilities
- **Collection**: 15+ collector types, RBAC integration, progress reporting
- **Redaction**: Regex-based, multiple redactor types, tracking/reporting  
- **Analysis**: 60+ analyzers, host+cluster analysis, structured results
- **Support Bundle**: Complete archiving, parsing, metadata system

### 6. Removed All Completion Markers
- **CHANGED**: All ``, `[ ]`, "" markers → `[ ]` (pending)
- **REASON**: Starting implementation from scratch despite existing foundation

### 7. Technical Approach Updates
- **Auto-collectors**: NEW package extending existing collection framework with dual-path approach
- **Redaction**: ENHANCE existing system with tokenization and streaming
- **Analysis**: WRAP existing analyzers with agent abstraction layer  
- **Diff**: COMPLETELY NEW capability using existing bundle parsing

### 8. Auto-Collectors Foundational Data Definition

**What "Foundational Data" Includes**:
- **Pods**: All pods in target namespace(s) with full spec and status
- **Deployments/ReplicaSets**: All deployment resources and their managed replica sets
- **Services**: All service definitions and endpoints
- **ConfigMaps**: All configuration data (with redaction)
- **Secrets**: All secret metadata (values redacted by default)
- **Events**: Recent cluster events for troubleshooting context
- **Pod Logs**: Container logs from all pods (with retention limits)
- **Image Facts**: Container image metadata (digests, tags, registry info)
- **Network Policies**: Any network policies affecting the namespace
- **RBAC**: Relevant roles, role bindings, service accounts

This foundational collection ensures that even without vendor-specific YAML specs, support bundles contain the essential data needed for troubleshooting most Kubernetes issues.

This updated PRD provides a realistic, implementable roadmap that leverages existing production-ready code while adding the new capabilities specified in the original requirements. The implementation risk is significantly reduced, and the timeline is more achievable.
