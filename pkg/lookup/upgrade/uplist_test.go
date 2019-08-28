package upgrade

import "testing"

func Test_isDevelName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "yay-git True", args: args{name: "yay-git"}, want: true},
		{name: "yay-svn True", args: args{name: "yay-svn"}, want: true},
		{name: "yay-nightly True", args: args{name: "yay-nightly"}, want: true},
		{name: "yay-bzr True", args: args{name: "yay-bzr"}, want: true},
		{name: "yay-hg True", args: args{name: "yay-hg"}, want: true},
		{name: "yay False", args: args{name: "yay"}, want: false},
		{name: "yay-bin False", args: args{name: "yay-bin"}, want: false},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDevelName(tt.args.name); got != tt.want {
				t.Errorf("isDevelName() = %v, want %v", got, tt.want)
			}
		})
	}
}
