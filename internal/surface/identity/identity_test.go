package identity

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnedSocketGenerationAndInvalidPaths(t *testing.T) {
	dir, err := os.MkdirTemp("/private/tmp", "abidentity-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "socket")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Socket(path)
	if err != nil || first == "" {
		t.Fatal(first, err)
	}
	listener.Close()
	listener, err = net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	second, err := Socket(path)
	if err != nil || second == first {
		t.Fatal(first, second, err)
	}
	regular := filepath.Join(dir, "regular")
	os.WriteFile(regular, []byte("not a socket"), 0600)
	link := filepath.Join(dir, "symlink")
	os.Symlink(path, link)
	for _, bad := range []string{"relative", regular, link, path + "\n"} {
		if _, err := Socket(bad); err == nil {
			t.Fatal("accepted", bad)
		}
	}
}
