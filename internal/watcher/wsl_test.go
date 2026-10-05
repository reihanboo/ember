package watcher

import "testing"

func TestIsWSLMount(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/mnt/c/project", want: true},
		{path: "/mnt/project", want: true},
		{path: "/mnt/", want: true},
		{path: `\mnt\c\project`, want: true},
		{path: "/mntfoo/project", want: false},
		{path: "/mnt2/project", want: false},
		{path: "/home/user/project", want: false},
		{path: "mnt/c/project", want: false},
		{path: "C:\\mnt\\project", want: false},
		{path: "", want: false},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := isWSLMount(test.path); got != test.want {
				t.Errorf("isWSLMount(%q) = %t, want %t", test.path, got, test.want)
			}
		})
	}
}
