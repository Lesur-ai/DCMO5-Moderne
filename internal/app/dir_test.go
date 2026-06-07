package app

import (
	"errors"
	"testing"
)

func TestStartMediaDir(t *testing.T) {
	errBoom := errors.New("boom")
	cases := []struct {
		name string
		wd   func() (string, error)
		home func() (string, error)
		want string
	}{
		{
			name: "répertoire courant prioritaire",
			wd:   func() (string, error) { return "/projets/dcmo5", nil },
			home: func() (string, error) { return "/home/clr", nil },
			want: "/projets/dcmo5",
		},
		{
			name: "repli HOME si getwd échoue",
			wd:   func() (string, error) { return "", errBoom },
			home: func() (string, error) { return "/home/clr", nil },
			want: "/home/clr",
		},
		{
			name: "repli HOME si getwd vide",
			wd:   func() (string, error) { return "", nil },
			home: func() (string, error) { return "/home/clr", nil },
			want: "/home/clr",
		},
		{
			name: "dernier repli « . » si tout échoue",
			wd:   func() (string, error) { return "", errBoom },
			home: func() (string, error) { return "", errBoom },
			want: ".",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := startMediaDir(c.wd, c.home); got != c.want {
				t.Errorf("startMediaDir() = %q, want %q", got, c.want)
			}
		})
	}
}
