package main

import "testing"

func TestSelectInstallMethod(t *testing.T) {
	cases := []struct {
		name          string
		gitSource     string
		installWheels string
		want          installMethod
		wantErr       bool
	}{
		{"pypi_default", "", "no", installMethodPypi, false},
		{"pypi_empty_install_wheels", "", "", installMethodPypi, false},
		{"wheels", "", "yes", installMethodWheels, false},
		{"git_no_wheels", "git+https://github.com/org/repo", "no", installMethodGit, false},
		{"git_empty_install_wheels", "git+https://github.com/org/repo@main", "", installMethodGit, false},
		{"conflict_git_and_wheels", "git+https://github.com/org/repo", "yes", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectInstallMethod(tc.gitSource, tc.installWheels)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for git+wheels conflict, got method=%v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("selectInstallMethod(%q, %q) = %v, want %v", tc.gitSource, tc.installWheels, got, tc.want)
			}
		})
	}
}
