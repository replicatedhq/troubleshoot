# Redaction Test Data

This directory contains test data and utilities for testing the redaction and tokenization system.

## Files

### `sample_secrets.yaml`
A comprehensive test file containing 200+ different types of secrets and sensitive data patterns:
- Environment variables (`DATABASE_PASSWORD=secret`)
- YAML key-value pairs (`password: "secret"`)
- Connection strings (`mysql://user:pass@host`)
- Nested YAML structures
- JSON environment variables
- AWS credentials
- API keys for various services (OpenAI, Stripe, GitHub, etc.)
- Database connection strings
- TLS certificates and private keys
- Docker registry credentials
- Monitoring service keys
- CI/CD tool tokens

### `test_sample_secrets.go`
A test program that demonstrates tokenization on the `sample_secrets.yaml` file.

### `sample_profiles.yaml` ⭐ **NEW**
A comprehensive test file specifically designed to demonstrate the redaction profile system. Contains carefully categorized test data for each profile level:

- **Minimal Profile Data**: Basic passwords, API keys, tokens, client secrets
- **Standard Profile Data**: + Email addresses, IP addresses, connection strings, service-specific secrets  
- **Comprehensive Profile Data**: + Usernames, hostnames, file paths, certificates, monitoring keys
- **Paranoid Profile Data**: + Long strings, UUIDs, base64 data, phone numbers, SSNs, credit cards
- **Non-Sensitive Data**: Regular config values that should NOT be redacted
- **Nested Structures**: Complex YAML with mixed sensitive/non-sensitive data

### `test_profiles_demo.go` ⭐ **NEW**
A comprehensive interactive demo program for the redaction profile system. This program:

- Tests all 4 built-in profiles (minimal, standard, comprehensive, paranoid)
- Shows profile escalation (how higher profiles catch more secrets)
- Analyzes token types and provides detailed statistics
- Saves redacted results for each profile to separate files
- Generates a comprehensive summary report
- Provides performance metrics and effectiveness ratings

## Usage

### Running the Test Program

```bash
# Navigate to the testdata directory
cd pkg/redact/testdata

# Test without tokenization (uses ***HIDDEN***)
go run test_sample_secrets.go

# Test with tokenization (generates unique tokens)
TROUBLESHOOT_TOKENIZATION=1 go run test_sample_secrets.go
```

### Testing the Profile System ⭐ **NEW**

```bash
# Navigate to the testdata directory
cd pkg/redact/testdata

# Test all redaction profiles with tokenization
TROUBLESHOOT_TOKENIZATION=1 go run test_profiles_demo.go
```

This will:
1. Load `sample_profiles.yaml` with categorized test data
2. Test all 4 profiles (minimal → standard → comprehensive → paranoid)
3. Show profile escalation and effectiveness ratings
4. Generate detailed statistics and token type analysis
5. Save redacted results for each profile to separate files
6. Create a comprehensive summary report

### Expected Results

**Without Tokenization:**
- Secrets are replaced with `***HIDDEN***`
- ~169 hidden values should be generated
- All critical secrets should be redacted

**With Tokenization:**
- Secrets are replaced with unique tokens like `***TOKEN_PASSWORD_ABC123***`
- ~169 unique tokens should be generated
- Token types include: PASSWORD, SECRET, TOKEN, USER, DATABASE, etc.
- Same values get same tokens (for correlation)
- Different values get different tokens

**Profile System Results:**
- **Minimal**: ~10-15 tokens (basic secrets only)
- **Standard**: ~25-35 tokens (+ IPs, emails, URLs)  
- **Comprehensive**: ~40-50 tokens (+ usernames, hostnames, paths)
- **Paranoid**: ~60+ tokens (+ long strings, UUIDs, phone numbers)

### Running Unit Tests

```bash
# Run all tokenization tests
go test ./pkg/redact -run TestTokenization -v

# Run comprehensive sample tests
go test ./pkg/redact -run TestComprehensiveSampleSecrets -v

# Run token type analysis
go test ./pkg/redact -run TestSampleSecretsTokenTypes -v
```

## What Gets Redacted

The redaction system detects and tokenizes:

1. **Environment Variables**: `KEY=value` format
2. **YAML Key-Value**: `key: "value"` format  
3. **Connection Strings**: Database URLs with credentials
4. **JSON Environment Variables**: Both escaped and unescaped formats
5. **Dynamic Patterns**: Service-specific keys (openai-key, stripe-secret, etc.)
6. **Nested Structures**: Complex YAML configurations
7. **Certificate Data**: TLS certificates and private keys

## Token Format

Tokens follow the format: `***TOKEN_<TYPE>_<HASH>***`

Examples:
- `***TOKEN_PASSWORD_A1B2C3***`
- `***TOKEN_SECRET_X7Y8Z9***`
- `***TOKEN_TOKEN_D4E5F6***`
- `***TOKEN_USER_G1H2I3***`
- `***TOKEN_DATABASE_J4K5L6***`

## Benefits

1. **Correlation**: Same secrets get same tokens across different files
2. **Security**: Original values are never exposed
3. **Debugging**: Can trace secret usage without seeing actual values
4. **Deterministic**: Same input always produces same tokens
5. **Type-Aware**: Different token prefixes help identify secret types
