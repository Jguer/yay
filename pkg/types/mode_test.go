package types

import "testing"

func TestTargetMode_IsAnyOrAUR(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		t    TargetMode
		want bool
	}{
		{name: "Aur", t: AUR, want: true},
		{name: "Any", t: Any, want: true},
		{name: "Repo", t: Repo, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.IsAnyOrAUR(); got != tt.want {
				t.Errorf("TargetMode.IsAnyOrAUR() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTargetMode_IsAnyOrRepo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		t    TargetMode
		want bool
	}{
		{name: "Aur", t: AUR, want: false},
		{name: "Any", t: Any, want: true},
		{name: "Repo", t: Repo, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.IsAnyOrRepo(); got != tt.want {
				t.Errorf("TargetMode.IsAnyOrRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTargetMode_IsAUR(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		t    TargetMode
		want bool
	}{
		{name: "Aur", t: AUR, want: true},
		{name: "Any", t: Any, want: false},
		{name: "Repo", t: Repo, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.IsAUR(); got != tt.want {
				t.Errorf("TargetMode.IsAUR() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTargetMode_IsRepo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		t    TargetMode
		want bool
	}{
		{name: "Aur", t: AUR, want: false},
		{name: "Any", t: Any, want: false},
		{name: "Repo", t: Repo, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.IsRepo(); got != tt.want {
				t.Errorf("TargetMode.IsRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}
