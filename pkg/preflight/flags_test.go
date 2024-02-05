package preflight

import (
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestAddFlagsString(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		want    string
		wantErr bool
	}{{
		name:    "expect error when non-existent flag is set",
		flag:    "non-existent",
		want:    "",
		wantErr: true,
	}, {
		name:    "expect format=human, err=nil when format flag is set",
		flag:    "format",
		want:    "human",
		wantErr: false,
	}, {
		name:    "expect collector-image=empty, err=nil when collector-image flag is set",
		flag:    "collector-image",
		want:    "",
		wantErr: false,
	}, {
		name:    "expect collector-pullpolicy=empty, err=nil when collector-pullpolicy flag is set",
		flag:    "collector-pullpolicy",
		want:    "",
		wantErr: false,
	}, {
		name:    "expect selector=empty, err=nil when selector flag is set",
		flag:    "selector",
		want:    "",
		wantErr: false,
	}, {
		name:    "expect since-time=empty, err=nil when since-time flag is set",
		flag:    "since-time",
		want:    "",
		wantErr: false,
	}, {
		name:    "expect since=empty, err=nil when since flag is set",
		flag:    "since",
		want:    "",
		wantErr: false,
	}, {
		name:    "expect output=empty, err=nil when output flag is set",
		flag:    "output",
		want:    "",
		wantErr: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := flag.FlagSet{}
			AddFlags(&f)

			got, err := f.GetString(tt.flag)

			assert.Equalf(t, tt.wantErr, err != nil, "AddFlags() error = %v, wantErr %v", err, tt.wantErr)
			assert.Equalf(t, got, tt.want, "AddFlags() = %v, wantErr %v", got, tt.want)
		})
	}
}

func TestAddFlagsBool(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		want    bool
		wantErr bool
	}{{
		name:    "expect interactive=true, err=nil when interactive flag is set",
		flag:    "interactive",
		want:    true,
		wantErr: false,
	}, {
		name:    "expect collect-without-permissions=true, err=nil when collect-without-permissions flag is set",
		flag:    "collect-without-permissions",
		want:    true,
		wantErr: false,
	}, {
		name:    "expect debug=true, err=nil when debug flag is set",
		flag:    "debug",
		want:    false,
		wantErr: false,
	}, {
		name:    "expect add-default-collectors=true, err=nil when add-default-collectors flag is set",
		flag:    "add-default-collectors",
		want:    true,
		wantErr: false,
	},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := flag.FlagSet{}
			AddFlags(&f)

			got, err := f.GetBool(tt.flag)

			assert.Equalf(t, tt.wantErr, err != nil, "AddFlags() error = %v, wantErr %v", err, tt.wantErr)
			assert.Equalf(t, got, tt.want, "AddFlags() = %v, wantErr %v", got, tt.want)
		})
	}
}
