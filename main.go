package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"os"

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

// these are memory mapped special registers
var special_registers = struct {
	KBSR, /* keyboard status register */
	KBDR uint16 /* keyboard data register */
}{
	0xFE00,
	0xFE02,
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
var stdinReader = bufio.NewReader(os.Stdin)

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		log.Println("lc3 [image-file1] ...")
		os.Exit(2)
	}

	for _, arg := range args {
		err := read_program(arg)
		if err != nil {
			log.Fatalln("failed to load image: %s\n", arg)
		}

	}

	defer restore_input_buffering()

	registers.COND = flags.ZRO
	registers.PC = 0x3000

	running := true
	for running {
		instr, err := mem_read(registers.PC)
		if err != nil {
			log.Fatalf("error reading instruction: %s\n",err)
		}

		op := *instr >> 12
		switch op {

		}
	}

}

func mem_read(addr uint16) (*uint16, error) {

	if addr == special_registers.KBSR {

		if ok, err := check_key(); err == nil && ok > 0 {

			special_registers.KBSR = 0x8000

			char, _, err := stdinReader.ReadRune()
			if err != nil {
				return nil, err
			}

			special_registers.KBDR = uint16(char)

		} else {
			special_registers.KBSR = 0
		}
	}

	return &memory[addr], nil
}

func check_key() (int, error) {
	var readFds unix.FdSet
	readFds.Zero()
	readFds.Set(int(os.Stdin.Fd()))

	var timeout unix.Timeval
	return unix.Select(1, &readFds, nil, nil, &timeout)
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

func disable_input_buffering() {
	termios.Tcgetattr(os.Stdin.Fd(), &orig_terminal_config)
	new_termios := orig_terminal_config
	new_termios.Lflag &^= unix.ICANON | unix.ECHO
	termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &new_termios)

}
func restore_input_buffering() {
	termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &orig_terminal_config)
}
