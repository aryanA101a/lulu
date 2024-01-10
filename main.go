package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
)

const Memory_Size = 1 << 16

var memory [Memory_Size]uint16

var registers struct {
	R0,
	R1,
	R2,
	R3,
	R4,
	R5,
	R6,
	R7,
	PC,
	COND,
	COUNT uint16
}

// opcodes
const (
	OP_BR int = iota
	OP_ADD
	OP_LD
	OP_ST
	OP_JSR
	OP_AND
	OP_LDR
	OP_STR
	OP_RTI
	OP_NOT
	OP_LDI
	OP_STI
	OP_JMP
	OP_RES
	OP_LEA
	OP_TRAP
)

var flags = struct {
	POS,
	ZRO,
	NEG uint16
}{
	1 << 0,
	1 << 1,
	1 << 2,
}

var orig_terminal_config unix.Termios

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		fmt.Println("lc3 [image-file1] ...")
		os.Exit(2)
	}

	for _, arg := range args {
		err := read_program(arg)
		if err != nil {
			fmt.Printf("failed to load image: %s\n", arg)
			os.Exit(1)
		}

	}

	//capture ctrl c and ctrl z
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		handle_interrupt()
	}()

	registers.COND = flags.ZRO
	registers.PC = 0x3000

	// select {}
}

func read_program(file_name string) error {
	file, err := os.ReadFile(file_name)
	if err != nil {
		return err
	}
	
	return read_program_file(&file)

}

func read_program_file(file *[]byte) error {
	if len(*file) < 4 {
		return fmt.Errorf("Error: File is too short.")
	}

	/* origin tells us where in memory to place the image
	convert to native endianess as lc3 programs are big endian */
	origin := int(binary.NativeEndian.Uint16((*file)[:2]))

	max_read := Memory_Size - int(origin)

	for i := 0; i < min(max_read, len(*file)); i += 2 {
		memory[origin+i] = binary.NativeEndian.Uint16((*file)[2+i : 2+i+2])
	}

	return nil

}

func handle_interrupt() {
	restore_input_buffering()
	fmt.Println("")
	os.Exit(-2)
}

func disable_input_buffering() {
	termios.Tcgetattr(os.Stdin.Fd(), &orig_terminal_config)
	new_termios := orig_terminal_config
	new_termios.Lflag &^= unix.ICANON | unix.ECHO
	termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &new_termios)

}
func restore_input_buffering() {
	termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &orig_terminal_config)
}
