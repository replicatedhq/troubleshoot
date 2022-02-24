// Based on https://github.com/kubernetes/apimachinery/blob/455a99f/pkg/util/intstr/intstr.go

package multitype

import (
	"testing"
)

func TestBoolOrString_Bool(t *testing.T) {
	type fields struct {
		Type    BoolOrStringType
		BoolVal bool
		StrVal  string
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{
		{
			name:    "expect true when BoolVal is true",
			fields:  fields{Type: Bool, BoolVal: true},
			want:    true,
			wantErr: false,
		}, {
			name:    "expect false when BoolVal is false",
			fields:  fields{Type: Bool, BoolVal: false},
			want:    false,
			wantErr: false,
		}, {
			name:    "expect false when StrVal is 'true'",
			fields:  fields{Type: String, StrVal: "true"},
			want:    true,
			wantErr: false,
		}, {
			name:    "expect false when StrVal is 'true'",
			fields:  fields{Type: String, StrVal: "true"},
			want:    true,
			wantErr: false,
		}, {
			name:    "expect false when StrVal is 'false'",
			fields:  fields{Type: String, StrVal: "false"},
			want:    false,
			wantErr: false,
		}, {
			name:    "expect false, error when StrVal is ''",
			fields:  fields{Type: String, StrVal: "''"},
			want:    false,
			wantErr: true,
		},
		{
			name:    "expect false, error when StrVal is '123'",
			fields:  fields{Type: String, StrVal: "123"},
			want:    false,
			wantErr: true,
		}, {
			name:    "expect true, nil when Type is not specified, StrVal is 'true'",
			fields:  fields{StrVal: "true"},
			want:    true,
			wantErr: false,
		}, {
			name:    "expect false, nil when Type is not specified, StrVal is 'false'",
			fields:  fields{StrVal: "false"},
			want:    false,
			wantErr: false,
		}, {
			name:    "expect false, nil when Type is not specified, StrVal is 'false' and BoolVal is true",
			fields:  fields{StrVal: "false", BoolVal: true},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boolstr := &BoolOrString{
				Type:    tt.fields.Type,
				BoolVal: tt.fields.BoolVal,
				StrVal:  tt.fields.StrVal,
			}
			got, err := boolstr.Bool()
			if (err != nil) != tt.wantErr {
				t.Errorf("BoolOrString.Bool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BoolOrString.Bool() = %v, want %v", got, tt.want)
			}
			if got := boolstr.BoolOrDefaultFalse(); got != tt.want {
				t.Errorf("BoolOrString.BoolOrDefaultFalse() = %v, want %v", got, tt.want)
			}
		})
	}
}
