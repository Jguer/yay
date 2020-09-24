package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLessRunes(t *testing.T) {
	t.Parallel()
	type args struct {
		iRunes []rune
		jRunes []rune
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "nilslices", args: args{iRunes: nil, jRunes: nil}, want: false},
		{name: "emptyslices", args: args{iRunes: []rune{}, jRunes: []rune{}}, want: false},
		{name: "simpleslice a,b", args: args{iRunes: []rune{'a'}, jRunes: []rune{'b'}}, want: true},
		{name: "simpleslice b,a", args: args{iRunes: []rune{'b'}, jRunes: []rune{'a'}}, want: false},
		{name: "equalslice", args: args{iRunes: []rune{'a', 'a', 'a'}, jRunes: []rune{'a', 'a', 'a'}}, want: false},
		{name: "uppercase", args: args{iRunes: []rune{'a'}, jRunes: []rune{'A'}}, want: false},
		{name: "longerFirstArg", args: args{iRunes: []rune{'a', 'b'}, jRunes: []rune{'a'}}, want: false},
		{name: "longerSecondArg", args: args{iRunes: []rune{'a'}, jRunes: []rune{'a', 'b'}}, want: true},
		{name: "utf8 less", args: args{iRunes: []rune{'世', '2', '0'}, jRunes: []rune{'世', '界', '3'}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LessRunes(tt.args.iRunes, tt.args.jRunes)
			assert.Equal(t, tt.want, got)
		})
	}
}
