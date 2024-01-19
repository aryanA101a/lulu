package vm

import (
	goIO "io"
	"log"
	"os"
	"time"

	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
)

type io struct {
	originalTerminalConfig unix.Termios
	stdoutWriter           goIO.Writer
	keyBuffer              chan byte
}

func (io *io) processKeyboard(memRead func(addr word) word, memWrite func(addr ,value word) ) {
	if (memRead(KBSR)&0x8000 == 0) && (len(io.keyBuffer) > 0) {
		memWrite(KBSR, memRead(KBSR)|0x8000)
		memWrite(KBDR, word(<-io.keyBuffer))

	}
}

func (io *io) pollKeyboard(memRead func(addr word) word) {
	ticker := time.NewTicker(5 * time.Millisecond)
	for range ticker.C {
		if memRead(KBSR)&0x8000 == 0 {
			buf := make([]byte, 1)
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				continue
			}

			for _, b := range buf[:n] {
				(*io).keyBuffer <- b
			}
		}
	}
}

// this configures the terminal to run in raw mode
func(io *io) enableRawMode() {
	log.Printf("enabling raw mode...")
	termios.Tcgetattr(os.Stdin.Fd(), &io.originalTerminalConfig)
	newTermios := io.originalTerminalConfig
	newTermios.Lflag &^= unix.ICANON | unix.ECHO
	termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &newTermios)

}

func(io *io) disableRawMode() {
	log.Printf("disabling raw mode...")
	termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &io.originalTerminalConfig)
}

func newIO() io {
	return io{
		stdoutWriter: goIO.Writer(os.Stdout),
		keyBuffer:    make(chan byte,1),
	}
}
