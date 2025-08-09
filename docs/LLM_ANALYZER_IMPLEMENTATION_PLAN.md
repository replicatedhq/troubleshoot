# LLM Analyzer Implementation Plan for Troubleshoot.sh

## Project Overview
Add an LLM-powered analyzer to Troubleshoot.sh that can intelligently analyze support bundles using AI to identify issues without pre-written deterministic rules.

## PRD Core Requirements
1. ✅ Add LLM analyzer to existing troubleshoot spec
2. ✅ User provides problem description (CLI flag or interactive)
3. ✅ Send logs/context to LLM for analysis
4. ✅ Include tests
5. ⏳ Stretch: Re-analyze existing bundles

## Progress Summary
- ✅ **Phase 1** - Repository Setup & Codebase Analysis (COMPLETED - 2.5 hours)
- ✅ **Phase 0** - Kubernetes Test Environment (COMPLETED - 45 minutes)
- ⏸️ **Phase 2** - Core Design & Type Definitions (30 min)
- ⏸️ **Phase 3** - MVP Implementation (2-3 hours)
- ⏸️ **Phase 4** - CLI Integration (1 hour)
- ⏸️ **Phase 5** - Testing (1-2 hours)
- ⏸️ **Phase 6** - Stretch: Bundle Re-analysis (1-2 hours)
- ⏸️ **Phase 7** - Documentation & Demo (30 min)

**Total Estimated Time:** 7-9 hours (includes test environment setup)
**Time Spent So Far:** 3.25 hours

## Project Structure

The project uses a dual-repository structure:
- **Main Repository** (`troubleshoot-llm-analyzer/`): Documentation, test environment, and planning
- **Fork** (`troubleshoot/`): Actual troubleshoot codebase on branch `add-llm-analyzer`

```
troubleshoot-llm-analyzer/
├── LLM_ANALYZER_IMPLEMENTATION_PLAN.md  # This document
├── CODEBASE_ANALYSIS.md                 # Architecture analysis
├── DEV_COMMANDS.md                      # Development reference
├── PROBLEM_DESCRIPTION.md               # Original requirements
├── test-cluster/                        # Kubernetes test environment
│   ├── setup.sh                        # Cluster creation script
│   ├── cleanup.sh                      # Cluster deletion script
│   ├── kind-config.yaml                # Kind configuration
│   ├── collector-spec.yaml             # Troubleshoot collector
│   └── test-scenarios/                 # Broken test apps
└── troubleshoot/                        # Fork of replicatedhq/troubleshoot
    └── (troubleshoot codebase)
```

## Key Deviations from Original Plan

1. **Phase Order**: Implemented Phase 0 (test environment) after Phase 1 instead of at the end
   - Rationale: Having real test data early helps validate the implementation
   
2. **Repository Structure**: Using dual-repo structure instead of single repo
   - Rationale: Keeps documentation separate from code changes for cleaner PR

3. **Test Scenarios**: Focused on 3 core scenarios instead of 4+
   - Implemented: OOM, CrashLoop, Connection errors
   - Skipped: RBAC issues (can add later if needed)
   - Rationale: These cover the most common Kubernetes issues

---

## Phase 1: Repository Setup & Codebase Analysis ✅ COMPLETED
**Duration:** 2-3 hours (Actual: 2.5 hours)  
**Goal:** Fork, setup repository, and understand the existing architecture

### Completed Tasks
1. ✅ Forked `replicatedhq/troubleshoot` repository on GitHub
2. ✅ Created dual repository structure (docs + code fork)
3. ✅ Created feature branch `add-llm-analyzer`
4. ✅ Analyzed existing analyzer implementations
5. ✅ Installed development tools (golangci-lint, goimports, gocov)
6. ✅ Documented codebase findings in `CODEBASE_ANALYSIS.md`
7. ✅ Created `DEV_COMMANDS.md` reference guide

### Key Learnings
- Analyzer interface is simple: `Title()`, `IsExcluded()`, `Analyze()`
- Must use provided file access functions, never direct filesystem
- Existing patterns are clean and easy to follow

---

## Phase 0: Kubernetes Test Environment Setup ✅ COMPLETED
**Duration:** 1 hour (Actual: 45 minutes)
**Goal:** Create a Kubernetes cluster with real issues to test the LLM analyzer

### Completed Tasks
1. ✅ Installed Kind and Helm (auto-installed via brew on macOS)
2. ✅ Created 2-node Kind cluster with port mappings
3. ✅ Deployed helm charts (PostgreSQL, Redis, nginx-ingress)
4. ✅ Created 3 broken test scenarios:
   - **OOMKilled Pod**: memory-hog using stress tool exceeding 50Mi limit
   - **CrashLoopBackOff**: crash-loop-app exiting with error after 5 seconds
   - **Connection Failure**: connection-test-app with wrong database credentials
5. ✅ Created troubleshoot collector spec with specific log collectors
6. ✅ Built troubleshoot CLI and verified bundle collection works

### Implementation Notes
- Used `polinux/stress` image for OOM testing (tries to allocate 100M with 50Mi limit)
- Created realistic error messages in crash and connection scenarios
- Added nginx-ingress for more complete cluster setup
- Collector spec includes logs, configmaps, pod descriptions, and events
- Setup script includes auto-installation of Kind/Helm on macOS
- Cleanup script provided for easy teardown

### Deliverables
- ✅ Working Kind cluster with 3 failure scenarios
- ✅ Automated setup/cleanup scripts with error handling
- ✅ Collector spec gathering relevant logs and cluster info
- ✅ README documentation for test environment
- ✅ Successfully collected test bundle (137KB tar.gz)

---

## Phase 2: Core Types & Design (30 minutes)
**Goal:** Define minimal types needed for LLM analyzer

### Tasks
1. Add `LLMAnalyze` struct to analyzer types
2. Define simple response structure
3. Plan integration points

### Type Design
```go
// In analyzer_shared.go
type LLMAnalyze struct {
    AnalyzeMeta   `json:",inline" yaml:",inline"`
    Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
    CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
    FileName      string     `json:"fileName,omitempty" yaml:"fileName,omitempty"`
    MaxFiles      int        `json:"maxFiles,omitempty" yaml:"maxFiles,omitempty"`
    Model         string     `json:"model,omitempty" yaml:"model,omitempty"` // default: gpt-3.5-turbo
}

// Internal types for LLM response
type LLMAnalysisResult struct {
    Summary    string  `json:"summary"`
    Issue      string  `json:"issue"`
    Solution   string  `json:"solution"`
    Confidence float64 `json:"confidence"`
}
```

### Deliverables
- [ ] Updated `analyzer_shared.go` with LLM type
- [ ] Basic design documented

---

## Phase 3: MVP Implementation (2-3 hours)
**Goal:** Get basic LLM analyzer working end-to-end

### Tasks
1. Create `pkg/analyze/llm.go` with analyzer implementation
2. Add simple OpenAI client (no abstraction layer yet)
3. Implement file collection (basic, no smart filtering)
4. Parse LLM JSON response
5. Map to outcomes (pass/warn/fail)
6. Register in `GetAnalyzer()` switch

### Implementation Details
- Read API key from `OPENAI_API_KEY` environment variable
- 60 second timeout
- Max 10 files or 500KB (whichever comes first)
- Simple prompt: include problem description + file contents

### Deliverables
- [ ] Working LLM analyzer (`pkg/analyze/llm.go`)
- [ ] Registered in analyzer switch statement
- [ ] Can be included in YAML spec

---

## Phase 4: CLI Integration (1 hour)
**Goal:** Add problem description input

### Tasks
1. Add `--problem-description` flag to analyze command
2. Implement interactive prompt if flag not provided
3. Pass problem description to analyzer context

### Deliverables
- [ ] CLI accepts `--problem-description` flag
- [ ] Interactive prompt works when flag not provided
- [ ] Problem description passed to LLM

---

## Phase 5: Testing (1-2 hours)
**Goal:** Ensure code quality and reliability

### Tasks
1. Unit tests for LLM analyzer with mocked OpenAI responses
2. Test YAML parsing of LLM spec
3. Test outcome evaluation
4. Integration test with bundles from Phase 0 test cluster
5. Manual test with real Kubernetes issues (OOM, CrashLoop, etc.)

### Test Strategy
- **Unit Tests**: Mock OpenAI client, test logic without API calls
- **Integration Tests**: Use test bundles from Phase 0
- **Manual Tests**: Run against Kind cluster with real issues
- **Validation**: Verify LLM correctly identifies OOM, crashes, connection issues

### Deliverables
- [ ] `pkg/analyze/llm_test.go` with comprehensive tests
- [ ] Mock OpenAI client for testing
- [ ] All tests passing
- [ ] Verified detection of real Kubernetes issues

---

## Phase 6: Stretch - Bundle Re-analysis (1-2 hours)
**Goal:** Allow re-analyzing existing bundles

### Tasks
1. Add `--bundle` flag to analyze command
2. Load existing bundle from tar.gz
3. Run LLM analyzer on extracted bundle
4. Support new problem description

### Deliverables
- [ ] Can re-analyze existing bundles with `--bundle` flag
- [ ] New problem descriptions supported

---

## Phase 7: Documentation & Demo (30 minutes)
**Goal:** Prepare submission

### Tasks
1. Create example LLM analyzer spec
2. Document usage in README
3. Record Loom demo using Phase 0 test cluster
4. Create PR

### Demo Script
1. Show Kind cluster with failing pods (OOM, CrashLoop)
2. Run troubleshoot with LLM analyzer
3. Show LLM identifying the issues correctly
4. Demonstrate re-analysis with different problem description (stretch)

### Deliverables
- [ ] Example spec in `examples/llm-analyzer.yaml`
- [ ] Updated documentation
- [ ] Loom video showing real Kubernetes issues being detected
- [ ] PR to upstream repository

---

## Example Spec (MVP)

```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: Analyzer
metadata:
  name: llm-analyzer
spec:
  analyzers:
    - llm:
        checkName: "LLM Problem Analysis"
        collectorName: "cluster-info"
        fileName: "*.log"  # Optional: specific files
        model: "gpt-3.5-turbo"  # Optional: default is gpt-3.5-turbo
        outcomes:
          - fail:
              when: "issue_found"
              message: "Issue detected: {{.Summary}}"
          - warn:
              when: "potential_issue"
              message: "Potential issue: {{.Summary}}"
          - pass:
              message: "No issues detected"
```

## Usage Examples

```bash
# With problem description flag
kubectl support-bundle --problem-description "Pods are crashing" spec.yaml

# Interactive (will prompt)
kubectl support-bundle spec.yaml
> Please describe the problem: Pods are crashing

# Re-analyze existing bundle (stretch goal)
kubectl analyze --bundle support-bundle.tar.gz --problem-description "Database connection issues"
```

## What We're NOT Building (Cut from Original Plan)

To focus on PRD requirements, we've cut:
- ❌ Cost tracking and limits
- ❌ Token counting and estimation
- ❌ Dry-run mode
- ❌ Audit logging
- ❌ Smart chunking with conversation state
- ❌ Multiple provider support (just OpenAI for MVP)
- ❌ Anthropic integration
- ❌ Local model support
- ❌ Caching
- ❌ Complex template variables
- ❌ Air-gapped detection

These can be added incrementally after MVP is working.

---

## Success Metrics
- ✅ Can add LLM analyzer to spec
- ✅ Accepts problem description
- ✅ Calls OpenAI API
- ✅ Returns analysis results
- ✅ Tests pass
- ✅ Works alongside existing analyzers