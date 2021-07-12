package collect

import (
	"testing"
)

func Test_checkValidLBAddress(t *testing.T) {
	type args struct {
		address string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Valid IP and Port",
			args: args{address: "1.2.3.4:6443"},
			want: true,
		},
		{
			name: "Valid domain and Port ",
			args: args{address: "replicated.com:80"},
			want: true,
		},
		{
			name: "Valid subdomain and Port ",
			args: args{address: "sub.replicated.com:80"},
			want: true,
		},
		{
			name: "Valid subdomain with '-' and Port ",
			args: args{address: "sub-domain.replicated.com:80"},
			want: true,
		},
		{
			name: "Special Character",
			args: args{address: "sw!$$.com:80"},
			want: false,
		},
		{
			name: "Too many characters",
			args: args{address: "howlongcanwemakethiswithoutrunningoutofwordsbecasueweneedtohitatleast64.com:80"},
			want: false,
		},
		{
			name: "Capital Letters",
			args: args{address: "testDomain.com:80"},
			want: false,
		},
		{
			name: "Invalid IP",
			args: args{address: "55.555.51.23:80"},
			want: false,
		},
		{
			name: "Too many consecutive .",
			args: args{address: "55..55.51.23:80"},
			want: false,
		},
		{
			name: "Invalid Port too low",
			args: args{address: "55.55.51.23:0"},
			want: false,
		},
		{
			name: "Invalid Port too large",
			args: args{address: "55.55.51.23:999990"},
			want: false,
		},
		{
			name: "Invalid Port Character",
			args: args{address: "55.55.51.23:port"},
			want: false,
		},
		{
			name: "Invalid Port Number",
			args: args{address: "55.55.51.23:32.5"},
			want: false,
		},
		{
			name: "Codes in addresses",
			args: args{address: "[34m192.168.0.1[00m"},
			want: false,
		}, {
			name: "Codes in addresses",
			args: args{address: "\033[34m192.168.0.1\033[00m\n "},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkValidLBAddress(tt.args.address); got != tt.want {
				t.Errorf("checkValidTCPAddress() = %v, want %v for %v", got, tt.want, tt.args.address)
			}
		})
	}
}

func Test_isIPCandidate(t *testing.T) {
	type args struct {
		address string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Basic IP4",
			args: args{address: "1.2.3.4"},
			want: true,
		},
		{
			name: "DNS name",
			args: args{address: "google.com"},
			want: false,
		},
		{
			name: "IP Jumble",
			args: args{address: "55.555.14.2"},
			want: true,
		},
		{
			name: "Invalid Address but good candidate",
			args: args{address: "55.5..3"},
			want: true,
		},
		{
			name: "Bad stuff",
			args: args{address: "t1232123.12123"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIPCandidate(tt.args.address); got != tt.want {
				t.Errorf("isIpCandidate() = %v, want %v", got, tt.want)
			}
		})
	}
}
