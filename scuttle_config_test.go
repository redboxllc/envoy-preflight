package main

import "testing"

func Test_replacePort(t *testing.T) {
	type args struct {
		sourceURL   string
		original    int
		replacement int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Happy flow",
			args: args{
				sourceURL:   "http://localhost:15000",
				original:    15000,
				replacement: 15020,
			},
			want: "http://localhost:15020",
		},
		{
			name: "Invalid URL",
			args: args{
				sourceURL:   "notaurl^^ :15000",
				original:    15000,
				replacement: 15020,
			},
			want: "",
		},
		{
			name: "Port not matching",
			args: args{
				sourceURL:   "http://localhost:14000",
				original:    15000,
				replacement: 15020,
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replacePort(tt.args.sourceURL, tt.args.original, tt.args.replacement); got != tt.want {
				t.Errorf("replacePort() = %v, want %v", got, tt.want)
			}
		})
	}
}
