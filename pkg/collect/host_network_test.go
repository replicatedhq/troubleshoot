package collect

import "testing"

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
			name: "valid1",
			args: args{address: "1.2.3.4:6443"},
			want: true,
		},
		{
			name: "valid2",
			args: args{address: "0.0.0.0:232"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkValidLBAddress(tt.args.address); got != tt.want {
				t.Errorf("checkValidTCPAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
