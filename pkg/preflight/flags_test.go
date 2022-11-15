package preflight

import (
	"testing"

	flag "github.com/spf13/pflag"
)

// Reset flags for preflightFlags
func resetFlags() {
	if preflightFlags != nil {
		preflightFlags = NewPreflightFlags()
	}
}

func TestAddFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		flags   []string
		want    []string
		wantErr bool
	}{{
		name: "expect output=result.txt, err=nil when output flag is result.txt",
		flags: []string{
			"output",
		},
		args: []string{
			"--output=result.txt",
		},
		want: []string{
			"result.txt",
		},
		wantErr: false,
	}, {
		name: "expect output=nil, err=nil when output flag is nil",
		flags: []string{
			"output",
		},
		args: []string{
			"--output=",
		},
		want: []string{
			"",
		},
		wantErr: false,
	}, {
		name: "expect output=result.txt, err=nil when o flag is result.txt",
		flags: []string{
			"output",
		},
		args: []string{
			"-o=result.txt",
		},
		want: []string{
			"result.txt",
		},
		wantErr: false,
	}, {
		name: "expect error when o flag is nil",
		flags: []string{
			"output",
		},
		args: []string{
			"-o",
		},
		want: []string{
			"",
		},
		wantErr: true,
	}, {
		name: "expect output=nil, err=nil when no output flag",
		flags: []string{
			"output",
		},
		args: []string{
			"",
		},
		want: []string{
			"",
		},
		wantErr: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			f := flag.FlagSet{}
			AddFlags(&f)

			if err := f.Parse(tt.args); err != nil {
				if (err != nil) != tt.wantErr {
					t.Errorf("AddFlags() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else {
				for i, flag := range tt.flags {
					got, err := f.GetString(flag)
					if (err != nil) != tt.wantErr {
						t.Errorf("AddFlags() error = %v, wantErr %v", err, tt.wantErr)
					}

					if got != tt.want[i] {
						t.Errorf("AddFlags() = %v, want %v", got, tt.want[i])
					}
				}
			}
		})
	}
}
