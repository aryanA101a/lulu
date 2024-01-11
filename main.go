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

const (
	R_R0 = iota
	R_R1
	R_R2
	R_R3
	R_R4
	R_R5
	R_R6
	R_R7
	R_PC
	R_COND
	R_COUNT
)

var registers = make([]uint16, 11)

// these are memory mapped special registers
var special_registers = struct {
	KBSR, /* keyboard status register */
	KBDR uint16 /* keyboard data register */
}{
	Memory_Mapped_Registers_Start,
	Memory_Mapped_Registers_Start + 0x0002,
}

const (
	Program_Memory_Start          = 0x3000
	Memory_Mapped_Registers_Start = 0xFE00
)

// opcodes
const (
	OP_BR uint16 = iota
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
			log.Fatalf("failed to load image: %s\n", arg)
		}

	}

	defer restore_input_buffering()

	registers[R_COND] = flags.ZRO
	registers[R_PC] = Program_Memory_Start

	running := true
	for running {
		instr, err := mem_read(registers[R_PC])
		if err != nil {
			log.Fatalf("error reading instruction: %s\n", err)
		}

		op := *instr >> 12

		switch op {

		case OP_ADD:
			dr := (*instr >> 9) & 0b111
			r1 := (*instr >> 6) & 0b111

			imm_flag := (*instr >> 5) & 0b1

			if imm_flag == 1 {
				imm5 := sign_extend((*instr)&0b11111, 5)
				registers[dr] = registers[r1] + imm5
			} else {
				r2 := (*instr) & 0b111
				registers[dr] = registers[r1] + registers[r2]
			}

			update_flags(dr)

		case OP_LDI:
			dr := (*instr >> 9) & 0b111
			pcoffset9 := sign_extend((*instr)&0x1FF, 9)

			data, err := mem_read(registers[R_PC] + pcoffset9)
			if err != nil {
				log.Fatalf("error executing LDI instruction: %s\n", err)
			}

			registers[dr] = *data
			update_flags(dr)

		}

	}

}

func update_flags(r0 uint16) {
	if registers[r0] == 0 {
		registers[R_COND] = flags.ZRO
	} else if registers[r0]>>15 == 1 {
		registers[R_COND] = flags.NEG
	} else {
		registers[R_COND] = flags.POS
	}
}

func sign_extend(x, bit_count uint16) uint16 {
	if (x >> (bit_count - 1) & 0b1) == 1 {
		x |= (0xFFFF << bit_count)
	}
	return x

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

// todo: handle program memory boundary
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
