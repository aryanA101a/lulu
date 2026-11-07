package vm

import (
	goIO "io"
	"log"
	"os"

	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
)

type io struct {
	originalTerminalConfig unix.Termios
	stdoutWriter           goIO.Writer
	stdinFD                int
	originalFileFlags      int
	inputFlagsSaved        bool
}

// processKeyboard leaves pending input untouched. Only traps wait for input.
func (io *io) processKeyboard(wait bool, memRead func(addr word) word, memWrite func(addr, value word)) bool {
	if memRead(KBSR)&0x8000 != 0 {
		return true
	}

	fd := io.stdinFD
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	timeout := 0
	if wait {
		timeout = -1
	}
	for {
		n, err := unix.Poll(fds, timeout)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			log.Printf("keyboard poll: %v", err)
			return false
		}
		if n == 0 {
			return false
		}
		events := fds[0].Revents
		if events&unix.POLLNVAL != 0 || events&unix.POLLIN == 0 {
			log.Printf("keyboard poll: input unavailable (events=%#x)", events)
			return false
		}
		var buf [1]byte
		n, err = unix.Read(fd, buf[:])
		if err == unix.EINTR  {
			continue
		}
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			if wait {
				continue
			}
			return false
		}
		if err == nil && n == 0 {
			err = goIO.EOF
		}
		if err != nil {
			log.Printf("keyboard read: %v", err)
			return false
		}
		memWrite(KBDR, word(buf[0]))
		memWrite(KBSR, memRead(KBSR)|0x8000)
		return true
	}
}

func (io *io) enableNonblockingInput() bool {
	flags, err := unix.FcntlInt(uintptr(io.stdinFD), unix.F_GETFL, 0)
	if err == nil {
		_, err = unix.FcntlInt(uintptr(io.stdinFD), unix.F_SETFL, flags|unix.O_NONBLOCK)
	}
	if err != nil {
		log.Printf("enable nonblocking input: %v", err)
		return false
	}
	io.originalFileFlags = flags
	io.inputFlagsSaved = true
	return true
}

func (io *io) restoreInputFlags() {
	if io.inputFlagsSaved {
		if _, err := unix.FcntlInt(uintptr(io.stdinFD), unix.F_SETFL, io.originalFileFlags); err != nil {
			log.Printf("restore input flags: %v", err)
		}
	}
}

// this configures the terminal to run in raw mode
func (io *io) enableRawMode() {
	log.Printf("enabling raw mode...")
	termios.Tcgetattr(os.Stdin.Fd(), &io.originalTerminalConfig)
	newTermios := io.originalTerminalConfig
	newTermios.Lflag &^= unix.ICANON | unix.ECHO
	termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &newTermios)

}

func (io *io) disableRawMode() {
	log.Printf("disabling raw mode...")
	termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &io.originalTerminalConfig)
}

func newIO() io {
	return io{
		stdoutWriter: goIO.Writer(os.Stdout),
		stdinFD:      int(os.Stdin.Fd()),
	}
}
