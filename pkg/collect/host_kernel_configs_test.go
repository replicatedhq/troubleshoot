package collect

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadKConfigsNoFiles(t *testing.T) {
	_, err := loadKConfigs("nonexistent-kernel-release")
	if err == nil {
		t.Errorf("loadKConfigs() error = %v, wantErr", err)
	}
}

func TestParseKConfigs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected KConfigs
	}{
		{
			name: "Valid input",
			input: `
#
# Automatically generated file; DO NOT EDIT.
# Linux/x86 6.5.0-1018-gcp Kernel Configuration
#
CONFIG_CC_VERSION_TEXT="x86_64-linux-gnu-gcc-12 (Ubuntu 12.3.0-1ubuntu1~22.04) 12.3.0"
CONFIG_CC_IS_GCC=y
CONFIG_GCC_VERSION=120300
CONFIG_CLANG_VERSION=0
CONFIG_AS_IS_GNU=y
CONFIG_AS_VERSION=23800
CONFIG_IKHEADERS=m`,
			expected: KConfigs{
				"CONFIG_CC_IS_GCC": kConfigBuiltIn,
				"CONFIG_AS_IS_GNU": kConfigBuiltIn,
				"CONFIG_IKHEADERS": kConfigAsModule,
			},
		},
		{
			name:     "Empty input",
			input:    "",
			expected: KConfigs{},
		},
		{
			name: "Invalid input",
			input: `
foobar
   CONFIG_AS_IS_GNU=y
CONFIG_IKHEADERS = m
			`,
			expected: KConfigs{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			configs, err := parseKConfigs(r)
			if err != nil {
				t.Fatalf("parseKConfigs() error = %v", err)
				return
			}
			if !reflect.DeepEqual(configs, tt.expected) {
				t.Errorf("parseKConfigs() = %v, want %v", configs, tt.expected)
			}
		})
	}
}
