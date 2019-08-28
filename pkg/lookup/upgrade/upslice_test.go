package upgrade

import "testing"

func TestUpSlice_Len(t *testing.T) {
	tests := []struct {
		name string
		u    UpSlice
		want int
	}{
		{
			name: "FourElementTest",
			u: UpSlice{
				{Name: "Up1", Repository: "aur", LocalVersion: "1.0.0", RemoteVersion: "1.0.1"},
				{Name: "Up2", Repository: "aur", LocalVersion: "1.0.0", RemoteVersion: "1.0.1"},
				{Name: "Up3", Repository: "aur", LocalVersion: "1.0.0", RemoteVersion: "1.0.1"},
				{Name: "Up4", Repository: "aur", LocalVersion: "1.0.0", RemoteVersion: "1.0.1"},
			},
			want: 4,
		},
		{
			name: "EmptyTest",
			u:    UpSlice{},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.Len(); got != tt.want {
				t.Errorf("UpSlice.Len() = %v, want %v", got, tt.want)
			}
		})
	}
}
