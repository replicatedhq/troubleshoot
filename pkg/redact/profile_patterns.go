package redact

// getMinimalPatterns returns patterns for the minimal profile
// Only the most critical secrets: passwords, API keys, tokens
func getMinimalPatterns() []RedactionPattern {
	return []RedactionPattern{
		// Basic password patterns
		{
			Name:        "yaml-password",
			Description: "Redact password values in YAML/JSON",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:password|pwd|pass)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `password|pwd|pass`,
			Severity:    "critical",
			Tags:        []string{"password", "yaml", "json"},
		},
		{
			Name:        "env-password",
			Description: "Redact password environment variables",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(^.*(?:password|pwd|pass).*=)(?P<mask>[^\s\n\r]+)(\s*$)`,
			Scan:        `password|pwd|pass`,
			Severity:    "critical",
			Tags:        []string{"password", "environment"},
		},

		// API Keys
		{
			Name:        "yaml-api-key",
			Description: "Redact API key values in YAML/JSON",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:api[-_]?key|apikey)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `key|api`,
			Severity:    "critical",
			Tags:        []string{"api-key", "yaml", "json"},
		},
		{
			Name:        "env-api-key",
			Description: "Redact API key environment variables",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(^.*(?:key|api|token).*=)(?P<mask>[^\s\n\r]+)(\s*$)`,
			Scan:        `key|api|token`,
			Severity:    "critical",
			Tags:        []string{"api-key", "environment"},
		},

		// Tokens
		{
			Name:        "yaml-token",
			Description: "Redact token values in YAML/JSON",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:token|auth[-_]?token|access[-_]?token)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `token`,
			Severity:    "critical",
			Tags:        []string{"token", "yaml", "json"},
		},

		// AWS Keys (multiline)
		{
			Name:          "aws-secret-access-key",
			Description:   "Redact AWS Secret Access Key in multiline JSON",
			Type:          "multi-line",
			Enabled:       true,
			SelectorRegex: `(?i)"name": *"[^\"]*SECRET_?ACCESS_?KEY[^\"]*"`,
			RedactorRegex: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			Severity:      "critical",
			Tags:          []string{"aws", "secret", "multiline"},
		},
		{
			Name:          "aws-access-key-id",
			Description:   "Redact AWS Access Key ID in multiline JSON",
			Type:          "multi-line",
			Enabled:       true,
			SelectorRegex: `(?i)"name": *"[^\"]*ACCESS_?KEY_?ID[^\"]*"`,
			RedactorRegex: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			Severity:      "high",
			Tags:          []string{"aws", "access-key", "multiline"},
		},
	}
}

// getStandardPatterns returns patterns for the standard profile
// Extends minimal with IPs, URLs, emails, and more secret types
func getStandardPatterns() []RedactionPattern {
	patterns := getMinimalPatterns()

	// Add standard-level patterns
	standardPatterns := []RedactionPattern{
		// Secrets
		{
			Name:        "yaml-secret",
			Description: "Redact secret values in YAML/JSON (including service-specific)",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:secret|secrets|.*[-_]?secret|.*[-_]?secrets)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `secret`,
			Severity:    "high",
			Tags:        []string{"secret", "yaml", "json", "dynamic"},
		},
		{
			Name:        "env-secret",
			Description: "Redact secret environment variables",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(^.*(?:secret|secrets).*=)(?P<mask>[^\s\n\r]+)(\s*$)`,
			Scan:        `secret`,
			Severity:    "high",
			Tags:        []string{"secret", "environment"},
		},

		// Client secrets and private keys
		{
			Name:        "yaml-client-secret",
			Description: "Redact client secret values in YAML/JSON",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:client[-_]?secret|client[-_]?key)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `client`,
			Severity:    "high",
			Tags:        []string{"client-secret", "oauth", "yaml", "json"},
		},
		{
			Name:        "yaml-private-key",
			Description: "Redact private key values in YAML/JSON",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:private[-_]?key|privatekey)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `private`,
			Severity:    "critical",
			Tags:        []string{"private-key", "crypto", "yaml", "json"},
		},

		// Database credentials
		{
			Name:        "yaml-database",
			Description: "Redact database name values in YAML/JSON",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:database|db|database[-_]?name|db[-_]?name)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `database|db`,
			Severity:    "medium",
			Tags:        []string{"database", "yaml", "json"},
		},
		{
			Name:        "env-database",
			Description: "Redact database environment variables",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(^.*(?:database|db).*=)(?P<mask>[^\s\n\r]+)(\s*$)`,
			Scan:        `database|db`,
			Severity:    "medium",
			Tags:        []string{"database", "environment"},
		},

		// Connection strings
		{
			Name:        "http-credentials",
			Description: "Redact connection strings with username and password",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(https?|ftp)(:\/\/)(?P<mask>[^:\"\/]+){1}(:)(?P<mask>[^@\"\/]+){1}(?P<host>@[^:\/\s\"]+){1}(?P<port>:[\d]+)?`,
			Scan:        `https?|ftp`,
			Severity:    "high",
			Tags:        []string{"connection-string", "http", "credentials"},
		},
		{
			Name:        "database-connection-string",
			Description: "Redact database connection strings with credentials",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `\b(\w*:\/\/)(?P<mask>[^:\"\/]*){1}(:)(?P<mask>[^:\"\/]*){1}(@)(?P<mask>[^:\"\/]*){1}(?P<port>:[\d]*)?(\/)(?P<mask>[\w\d\S-_]+){1}\b`,
			Scan:        `\b(\w*:\/\/)([^:\"\/]*){1}(:)([^@\"\/]*){1}(@)([^:\"\/]*){1}(:[\d]*)?(\/)([\w\d\S-_]+){1}\b`,
			Severity:    "high",
			Tags:        []string{"connection-string", "database", "credentials"},
		},

		// Email addresses
		{
			Name:        "yaml-email",
			Description: "Redact email values in YAML/JSON",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:email|mail|smtp[-_]?user|smtp[-_]?username)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `email|mail|smtp`,
			Severity:    "medium",
			Tags:        []string{"email", "pii", "yaml", "json"},
		},

		// IP addresses
		{
			Name:        "ipv4-addresses",
			Description: "Redact IPv4 addresses",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `\b(?P<mask>(?:[0-9]{1,3}\.){3}[0-9]{1,3})\b`,
			Scan:        `\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`,
			Severity:    "medium",
			Tags:        []string{"ip-address", "network", "pii"},
		},

		// YAML environment variable values
		{
			Name:        "yaml-env-values",
			Description: "Redact environment variable values in YAML",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*value\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `value`,
			Severity:    "medium",
			Tags:        []string{"environment", "yaml", "value"},
		},

		// JSON environment variables (unescaped)
		{
			Name:        "json-env-vars",
			Description: "Redact JSON environment variable values",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)("name":"[^"]*(?:password|secret|key|token)[^"]*","value":")(?P<mask>[^"]+)(")`,
			Scan:        `password|secret|key|token`,
			Severity:    "high",
			Tags:        []string{"json", "environment", "credentials"},
		},
	}

	return append(patterns, standardPatterns...)
}

// getComprehensivePatterns returns patterns for the comprehensive profile
// Extends standard with usernames, hostnames, file paths, and more
func getComprehensivePatterns() []RedactionPattern {
	patterns := getStandardPatterns()

	// Add comprehensive-level patterns
	comprehensivePatterns := []RedactionPattern{
		// Usernames
		{
			Name:        "yaml-username",
			Description: "Redact username values in YAML/JSON",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:username|user|userid|user[-_]?id)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `user`,
			Severity:    "medium",
			Tags:        []string{"username", "pii", "yaml", "json"},
		},
		{
			Name:        "env-username",
			Description: "Redact user environment variables",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(^.*(?:user|username|userid).*=)(?P<mask>[^\s\n\r]+)(\s*$)`,
			Scan:        `user`,
			Severity:    "medium",
			Tags:        []string{"username", "pii", "environment"},
		},

		// Hostnames and domains
		{
			Name:        "hostnames",
			Description: "Redact hostnames and domain names",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:host|hostname|server|domain)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `host|server|domain`,
			Severity:    "low",
			Tags:        []string{"hostname", "network", "infrastructure"},
		},

		// File paths (potentially sensitive)
		{
			Name:        "file-paths",
			Description: "Redact file paths that might contain sensitive information",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:path|file|directory|dir)\s*:\s*["\']?)(?P<mask>\/[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `path|file|directory`,
			Severity:    "low",
			Tags:        []string{"file-path", "filesystem"},
		},

		// URLs (comprehensive)
		{
			Name:        "urls",
			Description: "Redact URLs that might contain sensitive information",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:url|uri|endpoint)\s*:\s*["\']?)(?P<mask>https?:\/\/[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `url|uri|endpoint|https?`,
			Severity:    "medium",
			Tags:        []string{"url", "endpoint", "network"},
		},

		// Certificate data
		{
			Name:        "certificate-data",
			Description: "Redact certificate and key data",
			Type:        "literal",
			Enabled:     true,
			Match:       "-----BEGIN CERTIFICATE-----",
			Severity:    "high",
			Tags:        []string{"certificate", "crypto", "pki"},
		},
		{
			Name:        "private-key-data",
			Description: "Redact private key data",
			Type:        "literal",
			Enabled:     true,
			Match:       "-----BEGIN PRIVATE KEY-----",
			Severity:    "critical",
			Tags:        []string{"private-key", "crypto", "pki"},
		},

		// Database connection string formats
		{
			Name:        "sql-server-connection",
			Description: "Redact SQL Server connection string values",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(Data Source *= *)(?P<mask>[^\;]+)(;)`,
			Scan:        `data source`,
			Severity:    "medium",
			Tags:        []string{"database", "sql-server", "connection-string"},
		},
		{
			Name:        "mysql-connection",
			Description: "Redact MySQL connection string values",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(Server *= *)(?P<mask>[^\;]+)(;)`,
			Scan:        `server`,
			Severity:    "medium",
			Tags:        []string{"database", "mysql", "connection-string"},
		},

		// Service-specific patterns
		{
			Name:        "docker-registry-credentials",
			Description: "Redact Docker registry credentials",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:registry[-_]?password|docker[-_]?password)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `registry|docker`,
			Severity:    "high",
			Tags:        []string{"docker", "registry", "credentials"},
		},

		// Monitoring and observability
		{
			Name:        "monitoring-keys",
			Description: "Redact monitoring service API keys",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `(?i)(\s*(?:datadog|newrelic|sentry)[-_]?(?:api[-_]?key|key)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`,
			Scan:        `datadog|newrelic|sentry`,
			Severity:    "high",
			Tags:        []string{"monitoring", "api-key", "observability"},
		},
	}

	return append(patterns, comprehensivePatterns...)
}

// getParanoidPatterns returns patterns for the paranoid profile
// Maximum redaction - any potentially sensitive data
func getParanoidPatterns() []RedactionPattern {
	patterns := getComprehensivePatterns()

	// Add paranoid-level patterns
	paranoidPatterns := []RedactionPattern{
		// Any long alphanumeric strings (potential secrets)
		{
			Name:        "long-alphanumeric-strings",
			Description: "Redact any alphanumeric strings longer than 8 characters",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `\b(?P<mask>[A-Za-z0-9]{12,})\b`,
			Scan:        `[A-Za-z0-9]{12,}`,
			Severity:    "low",
			Tags:        []string{"paranoid", "alphanumeric", "potential-secret"},
		},

		// Any base64-like strings
		{
			Name:        "base64-strings",
			Description: "Redact potential base64 encoded data",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `\b(?P<mask>[A-Za-z0-9+/]{16,}={0,2})\b`,
			Scan:        `[A-Za-z0-9+/]{16,}={0,2}`,
			Severity:    "medium",
			Tags:        []string{"paranoid", "base64", "encoded"},
		},

		// UUIDs (might be sensitive identifiers)
		{
			Name:        "uuid-strings",
			Description: "Redact UUID strings",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `\b(?P<mask>[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})\b`,
			Scan:        `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`,
			Severity:    "low",
			Tags:        []string{"paranoid", "uuid", "identifier"},
		},

		// Any hex strings (potential hashes or keys)
		{
			Name:        "hex-strings",
			Description: "Redact long hexadecimal strings",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `\b(?P<mask>[0-9a-fA-F]{16,})\b`,
			Scan:        `[0-9a-fA-F]{16,}`,
			Severity:    "low",
			Tags:        []string{"paranoid", "hex", "hash", "potential-key"},
		},

		// Phone numbers
		{
			Name:        "phone-numbers",
			Description: "Redact phone numbers",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `\b(?P<mask>(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4})\b`,
			Scan:        `\+?1[-.\s]?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}`,
			Severity:    "medium",
			Tags:        []string{"paranoid", "phone", "pii"},
		},

		// Social Security Numbers (US format)
		{
			Name:        "ssn-numbers",
			Description: "Redact Social Security Numbers",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `\b(?P<mask>[0-9]{3}-[0-9]{2}-[0-9]{4})\b`,
			Scan:        `[0-9]{3}-[0-9]{2}-[0-9]{4}`,
			Severity:    "critical",
			Tags:        []string{"paranoid", "ssn", "pii", "sensitive"},
		},

		// Credit card numbers (basic pattern)
		{
			Name:        "credit-card-numbers",
			Description: "Redact credit card numbers",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `\b(?P<mask>(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13}|3[0-9]{13}|6(?:011|5[0-9]{2})[0-9]{12}))\b`,
			Scan:        `(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13}|3[0-9]{13}|6(?:011|5[0-9]{2})[0-9]{12})`,
			Severity:    "critical",
			Tags:        []string{"paranoid", "credit-card", "pii", "financial"},
		},

		// Any string that looks like a secret (common patterns)
		{
			Name:        "secret-like-patterns",
			Description: "Redact strings that look like secrets (sk_, ghp_, etc.)",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `\b(?P<mask>(?:sk_|ghp_|glpat-|xoxb-|AKIA)[A-Za-z0-9_-]{8,})\b`,
			Scan:        `(?:sk_|ghp_|glpat-|xoxb-|AKIA)[A-Za-z0-9_-]{8,}`,
			Severity:    "high",
			Tags:        []string{"paranoid", "secret-pattern", "api-key"},
		},

		// Any value in quotes that's longer than typical
		{
			Name:        "long-quoted-values",
			Description: "Redact long quoted values that might be sensitive",
			Type:        "single-line",
			Enabled:     true,
			Regex:       `("[A-Za-z0-9+/=_-]{20,}")`,
			Scan:        `"[A-Za-z0-9+/=_-]{20,}"`,
			Severity:    "low",
			Tags:        []string{"paranoid", "quoted", "long-value"},
		},
	}

	return append(patterns, paranoidPatterns...)
}
