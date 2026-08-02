package sync

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRelWithin(t *testing.T) {
	tests := []struct {
		name    string
		root    string
		path    string
		wantRel string
		wantOK  bool
	}{
		{name: "file directly under root", root: "/home/user/watch", path: "/home/user/watch/report.pdf", wantRel: "report.pdf", wantOK: true},
		{name: "root itself", root: "/home/user/watch", path: "/home/user/watch", wantRel: ".", wantOK: true},
		{name: "nested file", root: "/home/user/watch", path: "/home/user/watch/sub/deep.txt", wantRel: filepath.Join("sub", "deep.txt"), wantOK: true},
		{name: "trailing separator", root: "/home/user/watch", path: "/home/user/watch/sub/", wantRel: "sub", wantOK: true},
		{name: "parent directory", root: "/home/user/watch", path: "/home/user", wantRel: "", wantOK: false},
		{name: "sibling with shared prefix", root: "/home/user/watch", path: "/home/user/watch2/x.txt", wantRel: "", wantOK: false},
		{name: "dot-dot escape", root: "/home/user/watch", path: "/home/user/watch/../secret", wantRel: "", wantOK: false},
		{name: "dot-dot escape two levels", root: "/home/user/watch", path: "/home/user/watch/a/../../secret", wantRel: "", wantOK: false},
		{name: "unrelated absolute path", root: "/home/user/watch", path: "/etc/passwd", wantRel: "", wantOK: false},
		{name: "drive root-like path", root: "/", path: "/etc/passwd", wantRel: filepath.FromSlash("etc/passwd"), wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRel, gotOK := RelWithin(tt.root, tt.path)
			if gotOK != tt.wantOK || gotRel != tt.wantRel {
				t.Fatalf("RelWithin(%q, %q) = (%q, %v), want (%q, %v)", tt.root, tt.path, gotRel, gotOK, tt.wantRel, tt.wantOK)
			}
		})
	}
}

func TestResolveRoot(t *testing.T) {
	tests := []struct {
		name     string
		roots    []string
		path     string
		wantRoot string
		wantOK   bool
	}{
		{name: "single root", roots: []string{"/home/user/watch"}, path: "/home/user/watch/a.txt", wantRoot: "/home/user/watch", wantOK: true},
		{name: "root itself", roots: []string{"/home/user/watch"}, path: "/home/user/watch", wantRoot: "/home/user/watch", wantOK: true},
		{name: "nested roots keep order-first semantics", roots: []string{"/home/user/watch", "/home/user/watch/sub"}, path: "/home/user/watch/sub/a.txt", wantRoot: "/home/user/watch", wantOK: true},
		{name: "second root matches", roots: []string{"/home/user/a", "/home/user/b"}, path: "/home/user/b/x.txt", wantRoot: "/home/user/b", wantOK: true},
		{name: "no root contains path", roots: []string{"/home/user/watch"}, path: "/tmp/other", wantRoot: "", wantOK: false},
		{name: "no roots", roots: nil, path: "/tmp/other", wantRoot: "", wantOK: false},
		{name: "sibling with shared prefix is not contained", roots: []string{"/home/user/watch"}, path: "/home/user/watch2/x.txt", wantRoot: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRoot, gotOK := ResolveRoot(tt.roots, tt.path)
			if gotOK != tt.wantOK || gotRoot != tt.wantRoot {
				t.Fatalf("ResolveRoot(%v, %q) = (%q, %v), want (%q, %v)", tt.roots, tt.path, gotRoot, gotOK, tt.wantRoot, tt.wantOK)
			}
		})
	}
}

func TestSafeJoin(t *testing.T) {
	tests := []struct {
		name    string
		root    string
		remote  string
		want    string
		wantErr bool
	}{
		{name: "plain name", root: "/watch", remote: "report.pdf", want: filepath.Join("/watch", "report.pdf")},
		{name: "name with spaces", root: "/watch", remote: "my report 2024.pdf", want: filepath.Join("/watch", "my report 2024.pdf")},
		{name: "unicode name", root: "/watch", remote: "café éclair 🎉.txt", want: filepath.Join("/watch", "café éclair 🎉.txt")},
		{name: "long name", root: "/watch", remote: strings.Repeat("a", 200) + ".txt", want: filepath.Join("/watch", strings.Repeat("a", 200)+".txt")},
		{name: "backslash is a literal on linux and stays inside the root", root: "/watch", remote: `a\b.txt`, want: filepath.Join("/watch", `a\b.txt`)},
		{name: "empty name", root: "/watch", remote: "", wantErr: true},
		{name: "dot name", root: "/watch", remote: ".", wantErr: true},
		{name: "dot-dot name", root: "/watch", remote: "..", wantErr: true},
		{name: "traversal", root: "/watch", remote: "../evil", wantErr: true},
		{name: "embedded separator is rejected", root: "/watch", remote: "a/b", wantErr: true},
		{name: "absolute name is rejected", root: "/watch", remote: "/etc/passwd", wantErr: true},
		{name: "windows device name con", root: "/watch", remote: "CON", wantErr: true},
		{name: "windows device name with extension", root: "/watch", remote: "con.txt", wantErr: true},
		{name: "windows device name nul", root: "/watch", remote: "NUL", wantErr: true},
		{name: "windows device name com1", root: "/watch", remote: "COM1", wantErr: true},
		{name: "windows device name lpt9", root: "/watch", remote: "lpt9.doc", wantErr: true},
		{name: "com10 is not a reserved device", root: "/watch", remote: "COM10", want: filepath.Join("/watch", "COM10")},
		{name: "com1x is not a reserved device", root: "/watch", remote: "COM1x", want: filepath.Join("/watch", "COM1x")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeJoin(tt.root, tt.remote)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SafeJoin(%q, %q) = %q, want error", tt.root, tt.remote, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("SafeJoin(%q, %q) unexpected error: %v", tt.root, tt.remote, err)
			}
			if got != tt.want {
				t.Fatalf("SafeJoin(%q, %q) = %q, want %q", tt.root, tt.remote, got, tt.want)
			}
			rel, ok := RelWithin(tt.root, got)
			if !ok || rel == "." {
				t.Fatalf("SafeJoin(%q, %q) = %q escapes the root (RelWithin ok=%v rel=%q)", tt.root, tt.remote, got, ok, rel)
			}
		})
	}
}

func TestIsTempFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/watch/.hidden", want: true},
		{path: "/watch/.git/config", want: false}, // base name is "config"; .git is handled by ignore rules
		{path: "/watch/file.swp", want: true},
		{path: "/watch/file.swx", want: true},
		{path: "/watch/file.tmp", want: true},
		{path: "/watch/file.part", want: true},
		{path: "/watch/file.crdownload", want: true},
		{path: "/watch/file~", want: true},
		{path: "/watch/report.pdf", want: false},
		{path: "/watch/report.txt", want: false},
		{path: "/watch/tmpfile", want: false},
		{path: "/watch/", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsTempFile(tt.path); got != tt.want {
				t.Fatalf("IsTempFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDriveErrorMessage(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{msg: "googleapi: Error 400: Invalid request", want: "path too deep: Drive limit is 100 nested folders"},
		{msg: "googleapi: Error 403: Forbidden", want: "path too deep: Drive limit is 100 nested folders"},
		{msg: "network timeout", want: "network timeout"},
		{msg: "drive.ListFiles: connection refused", want: "drive.ListFiles: connection refused"},
		{msg: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			if got := DriveErrorMessage(tt.msg); got != tt.want {
				t.Fatalf("DriveErrorMessage(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}
