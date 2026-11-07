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
	keyBuffer              chan byte
}

// processKeyboard stages input without overwriting an unconsumed character.
// Only input traps wait for a character; the CPU loop must keep executing.
func (io *io) processKeyboard(wait bool, memRead func(addr word) word, memWrite func(addr, value word)) {
	if memRead(KBSR)&0x8000 != 0 {
		return
	}

	var c byte
	if wait {
		c = <-io.keyBuffer
	} else {
		select {
		case c = <-io.keyBuffer:
		default:
			return
		}
	}
	memWrite(KBDR, word(c))
	memWrite(KBSR, memRead(KBSR)|0x8000)
}

// The reader owns host input; only the CPU goroutine accesses device registers.
func (io *io) pollKeyboard() {
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		for _, b := range buf[:n] {
			io.keyBuffer <- b
		}
		if err != nil {
			return
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
		keyBuffer:    make(chan byte, 1),
	}
}
