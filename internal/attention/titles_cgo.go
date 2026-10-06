//go:build cgo

package attention

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>
*/
import "C"
import (
	"os"
	"path/filepath"
	"unsafe"
)

func readTitle(path, query, id string) string {
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	var db *C.sqlite3
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	if C.sqlite3_open_v2(p, &db, C.SQLITE_OPEN_READONLY, nil) != C.SQLITE_OK {
		if db != nil {
			C.sqlite3_close(db)
		}
		return ""
	}
	defer C.sqlite3_close(db)
	C.sqlite3_busy_timeout(db, 5)
	q := C.CString(query)
	defer C.free(unsafe.Pointer(q))
	var stmt *C.sqlite3_stmt
	if C.sqlite3_prepare_v2(db, q, -1, &stmt, nil) != C.SQLITE_OK {
		return ""
	}
	defer C.sqlite3_finalize(stmt)
	value := C.CString(id)
	defer C.free(unsafe.Pointer(value))
	C.sqlite3_bind_text(stmt, 1, value, -1, nil)
	if C.sqlite3_step(stmt) != C.SQLITE_ROW {
		return ""
	}
	text := C.sqlite3_column_text(stmt, 0)
	if text == nil {
		return ""
	}
	return CleanTitle(C.GoString((*C.char)(unsafe.Pointer(text))))
}
func codexTitle(home, id string) string {
	if title := readTitle(filepath.Join(home, ".codex", "sqlite", "codex-dev.db"), "SELECT display_title FROM local_thread_catalog WHERE thread_id=? AND host_id='local' LIMIT 1", id); title != "" {
		return title
	}
	return readTitle(filepath.Join(home, ".codex", "state_5.sqlite"), "SELECT title FROM threads WHERE id=? LIMIT 1", id)
}
