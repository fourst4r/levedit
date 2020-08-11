package main

import (
	"image/color"
	"reflect"
	"testing"
)

func Test_coltof3(t *testing.T) {
	type args struct {
		c color.Color
	}
	tests := []struct {
		name string
		args args
		want [3]float32
	}{
		{
			args: args{color.RGBA{127, 127, 127, 255}},
			want: [3]float32{0.49803922, 0.49803922, 0.49803922},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := coltof3(tt.args.c); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("coltof3() = %v, want %v", got, tt.want)
			}
		})
	}
}
