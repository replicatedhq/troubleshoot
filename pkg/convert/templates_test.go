package convert

import "testing"

func TestExecute(t *testing.T) {
	type args struct {
		text string
		data interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "{{repl",
			args: args{
				text: "{{repl printf \"%s\" \"hello\"}}",
				data: nil,
			},
			want: "hello",
		},
		{
			name: "repl{{",
			args: args{
				text: "repl{{ printf \"%s\" \"hello\"}}",
				data: nil,
			},
			want: "hello",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Execute(tt.args.text, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Execute() = %v, want %v", got, tt.want)
			}
		})
	}
}
