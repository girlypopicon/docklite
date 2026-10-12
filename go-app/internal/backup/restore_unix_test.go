package backup

import (
	"os"
	"syscall"
)

func statIno(st os.FileInfo) uint64 { return st.Sys().(*syscall.Stat_t).Ino }
