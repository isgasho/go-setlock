package setlock

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

const (
	INVALID_FILE_HANDLE       = ^syscall.Handle(0)
	LOCKFILE_EXCLUSIVE_LOCK   = 0x0002
	LOCKFILE_FAIL_IMMEDIATELY = 0x0001
)

var (
	modkernel32    = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx = modkernel32.NewProc("LockFileEx")
)

type locker struct {
	nonblock bool
	filename string
	fd       syscall.Handle
}

func NewLocker(filename string, nonblock bool) *locker {
	return &locker{
		nonblock: nonblock,
		filename: filename,
		fd:       INVALID_FILE_HANDLE,
	}
}

func (l *locker) Lock() {
	if err := LockWithErr(); err != nil {
		panic(err)
	}
}

func (l *locker) LockWithErr() error {
	if l.fd != INVALID_FILE_HANDLE {
		return ErrFailedToAcquireLock
	}

	var flags uint32
	if l.nonblock {
		flags = LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY
	} else {
		flags = LOCKFILE_EXCLUSIVE_LOCK
	}

	if l.filename == "" {
		return ErrLockFileEmpty
	}
	fd, err := syscall.CreateFile(&(syscall.StringToUTF16(l.filename)[0]), syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return fmt.Errorf("setlock: fatal: unable to open %s: temporary failure", l.filename)
	}

	if fd == INVALID_FILE_HANDLE {
		return ErrFailedToAcquireLock
	}
	defer func() {
		// Close this descriptor if we failed to lock
		if l.fd == INVALID_FILE_HANDLE {
			// l.fd is not set, I guess we didn't suceed
			syscall.CloseHandle(fd)
		}
	}()

	var ol syscall.Overlapped
	var mu sync.RWMutex
	mu.Lock()
	defer mu.Unlock()

	r1, _, _ := syscall.Syscall6(
		procLockFileEx.Addr(),
		6,
		uintptr(fd), // handle
		uintptr(flags),
		uintptr(0), // reserved
		uintptr(1), // locklow
		uintptr(0), // lockhigh
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		return ErrFailedToAcquireLock
	}

	l.fd = fd
	return nil
}

func (l *locker) Unlock() {
	if fd := l.fd; fd != INVALID_FILE_HANDLE {
		syscall.CloseHandle(fd)
	}
}
