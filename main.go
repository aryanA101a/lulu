package main

import (
	"bufio"
	"context"
	"time"

	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
)

const Memory_Size = 1 << 16

var memory [Memory_Size]uint16

const (
	R0 = iota
	R1
	R2
	R3
	R4
	R5
	R6
	R7
	PC //Program Counter
	// PSR //Processor Status Register
	COUNT
	COND
)

var register = make([]uint16, 12)

var memory_mapped_register = struct {
	KBSR, /* keyboard status register */
	KBDR uint16 /* keyboard data register */
}{
	Memory_Mapped_Registers_Start,
	Memory_Mapped_Registers_Start + 0x0002,
}

var saved_stack_pointer_register = struct {
	SSP, /* supervisor stack pointer */
	USP uint16 /* user stack pointer */
}{
	0x2000,
	0xc000,
}

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

const (
	Trap_Vector_Table_Start       = 0x0000
	Interrupt_Vector_Table_Start  = 0x0100
	System_Space_Start            = 0x0200
	User_Space_Start              = 0x3000
	Memory_Mapped_Registers_Start = 0xFE00
)

const (
	TRAP_GETC  uint16 = 0x20 /* get character from keyboard, not echoed onto the terminal */
	TRAP_OUT   uint16 = 0x21 /* output a character */
	TRAP_PUTS  uint16 = 0x22 /* output a word string */
	TRAP_IN    uint16 = 0x23 /* get character from keyboard, echoed onto the terminal */
	TRAP_PUTSP uint16 = 0x24 /* output a byte string */
	TRAP_HALT  uint16 = 0x25 /* halt the program */
)

var orig_terminal_config unix.Termios
var stdinReader = bufio.NewReader(os.Stdin)
var stdoutWriter = io.Writer(os.Stdout)
var keyBuffer = make(chan byte, 1)

func main() {
	// f, err := os.OpenFile("logs.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	// if err != nil {
	// 	log.Fatalf("error opening file: %v", err)
	// }
	// defer f.Close()
	// log.SetOutput(f)

	args := os.Args[1:]
	if len(args) < 1 {
		//log.Println("lc3 [image-file1] ...")
		os.Exit(2)
	}
	for _, arg := range args {
		err := read_program(arg)
		if err != nil {
			log.Panicf("failed to load image: %s\n", arg)
		}

	}
	disable_input_buffering()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	defer restore_input_buffering()

	go poll_keyboard()

	// register[PSR] = 0x8000 | flags.ZRO
	register[COND] = flags.ZRO
	register[PC] = User_Space_Start

	running := true
	for running {
		process_keyboard()
		instr := mem_read(register[PC])
		//log.Printf("pc: %04x instr %04x\n", register[PC], instr)

		register[PC]++
		op := instr >> 12

		switch op {

		case OP_ADD:
			//log.Println("add")
			dr := (instr >> 9) & 0b111
			r1 := (instr >> 6) & 0b111

			imm_flag := (instr >> 5) & 0b1

			if imm_flag == 1 {
				imm5 := sext((instr)&0x1F, 5)
				register[dr] = register[r1] + imm5
			} else {
				r2 := (instr) & 0b111
				register[dr] = register[r1] + register[r2]
			}

			update_flags(dr)

		case OP_AND:
			//log.Println("and")

			dr := (instr >> 9) & 0b111
			sr1 := (instr >> 6) & 0b111

			imm_flag := (instr >> 5) & 0b1

			if imm_flag == 1 {
				imm5 := sext((instr)&0b11111, 5)
				register[dr] = register[sr1] & imm5
			} else {
				sr2 := (instr) & 0b111
				register[dr] = register[sr1] & register[sr2]
			}

			update_flags(dr)

		case OP_NOT:
			//log.Println("not")

			dr := (instr >> 9) & 0b111
			sr := (instr >> 6) & 0b111

			register[dr] = ^register[sr]
			update_flags(dr)

		case OP_BR:
			// log.Println("br")

			nzp := (instr >> 9) & 0b111
			pcoffset9 := (instr) & 0x1FF
			cond := register[COND]

			// log.Printf("%d br %04x", nzp&cond, register[PC]+sext(pcoffset9, 9))
			// log.Panicf("br %08b %04x\n", nzp, pcoffset9)

			if (nzp & cond) != 0 {
				register[PC] += sext(pcoffset9, 9)
			}

		case OP_JMP:
			//log.Println("jmp")

			br := ((instr) >> 6) & 0b111
			register[PC] = register[br]

		case OP_JSR:
			//log.Println("jsr")

			register[R7] = register[PC]

			bit11 := (instr >> 11) & 0b1
			if bit11 == 1 {
				pcoffset11 := (instr) & 0x7FF
				register[PC] += sext(pcoffset11, 11)
			} else {
				br := (instr >> 6) & 0b111
				register[PC] = register[br]
			}
		case OP_LD:
			//log.Println("ld")

			dr := (instr >> 9) & 0b111
			pcoffset9 := (instr) & 0x1FF

			register[dr] = mem_read(register[PC] + sext(pcoffset9, 9))
			update_flags(dr)

		case OP_LDI:

			dr := (instr >> 9) & 0b111
			pcoffset9 := (instr) & 0x1FF
			// log.Printf("ldi")
			register[dr] = mem_read(mem_read(register[PC] + sext(pcoffset9, 9)))
			update_flags(dr)

		case OP_LDR:
			//log.Println("ldr")

			dr := (instr >> 9) & 0b111
			br := (instr >> 6) & 0b111
			pcoffset6 := (instr) & 0x3F

			register[dr] = mem_read(register[br] + sext(pcoffset6, 6))
			update_flags(dr)

		case OP_LEA:
			//log.Println("lea")

			dr := (instr >> 9) & 0b111
			pcoffset9 := (instr) & 0x1FF

			register[dr] = register[PC] + sext(pcoffset9, 9)
			update_flags(dr)

		case OP_RTI:
			//log.Println("rti")

			// if register[PSR]>>15 == 0 {

			// 	addr, err := mem_read(register[R6])
			// 	if err != nil {
			// 		  log.Panicf("error executing RTI instruction: %s\n", err)
			// 	}
			// 	register[PC] = *addr

			// 	register[R6]++

			// 	saved_psr, err := mem_read(register[R6])
			// 	if err != nil {
			// 		  log.Panicf("error executing RTI instruction: %s\n", err)
			// 	}

			// 	register[R6]++

			// 	register[PSR] = *saved_psr

			// 	if register[PSR]>>15 == 1 {
			// 		saved_stack_pointer_register.SSP = register[R6]
			// 		register[R6] = saved_stack_pointer_register.USP
			// 	}

			// } else {
			// 	//privilage mode exception
			// }

		case OP_ST:
			//log.Println("st")

			sr := (instr >> 9) & 0b111
			pcoffset9 := (instr) & 0x1FF
			computed_address := register[PC] + sext(pcoffset9, 9)

			// if (computed_address < User_Space_Start && computed_address >= System_Space_Start) &&
			// 	register[PSR]>>15 == 1 {
			//acv exception
			// } else {
			memory[computed_address] = register[sr]
			// }
		case OP_STI:
			//log.Println("sti")

			sr := (instr >> 9) & 0b111
			pcoffset9 := (instr) & 0x1FF

			// if (computed_address < User_Space_Start && computed_address >= System_Space_Start) &&
			// 	register[PSR]>>15 == 1 {
			//acv exception
			// } else {
			memory[mem_read(register[PC]+sext(pcoffset9, 9))] = register[sr]
			// }

		case OP_STR:
			//log.Println("str")

			sr := (instr >> 9) & 0b111
			br := (instr >> 6) & 0b111
			pcoffset6 := (instr) & 0x3F
			computed_address := register[br] + sext(pcoffset6, 6)

			// if (computed_address < User_Space_Start && computed_address >= System_Space_Start) &&
			// 	register[PSR]>>15 == 1 {
			// 	//acv exception
			// } else {
			memory[computed_address] = register[sr]
			// }

		case OP_TRAP:
			// log.Printf("trap %04x", instr&0xFF)

			// temp := register[PSR]
			// if register[PSR]>>15 == 1 {
			// 	saved_stack_pointer_register.USP = register[R6]
			// 	register[R6] = saved_stack_pointer_register.SSP
			// 	register[PSR] |= 0x8000
			// }
			// register[R6]++
			// memory[register[R6]] = temp
			// register[R6]++
			// memory[register[R6]] = register[PC]

			// register[PC] = mem_read(zext(trapvect8))

			switch instr & 0xFF {
			case TRAP_GETC:
				// char, err := stdinReader.ReadByte()
				// if err != nil {
				// 	log.Panicf("error executing trap GETC: %s\n", err)
				// }
				//log.Printf("%0x %c",char,char)
				register[R0] = uint16(<-keyBuffer)
				update_flags(R0)

			case TRAP_OUT:
				c := byte(register[R0])
				_, err := stdoutWriter.Write([]byte{c})
				if err != nil {
					log.Panicf("error executing trap OUT: %s\n", err)
				}

			case TRAP_PUTS:
				temp := register[R0]

				for c := mem_read(temp) & 0xFFFF; c != 0; {
					fmt.Printf("%c", rune(c))
					temp++
					c = mem_read(temp) & 0xFFFF
				}

			case TRAP_IN:
				_, err := stdoutWriter.Write([]byte("Enter a character: "))
				if err != nil {
					log.Panicf("error executing trap IN: %s\n", err)
				}
				c := <-keyBuffer
				// if err != nil {
				// 	log.Panicf("error executing trap IN: %s\n", err)
				// }
				_, err = stdoutWriter.Write([]byte{c})
				if err != nil {
					log.Panicf("error executing trap IN: %s\n", err)
				}
				register[R0] = uint16(c)
				update_flags(R0)

			case TRAP_PUTSP:
				temp := register[R0]

				for word := mem_read(temp); word != 0; temp++ {
					_, err := stdoutWriter.Write([]byte{byte(word)})
					if err != nil {
						log.Panicf("error executing trap PUSHSP: %s\n", err)
					}
					if word>>8 != 0 {
						_, err := stdoutWriter.Write([]byte{byte(word >> 8)})
						if err != nil {
							log.Panicf("error executing trap PUSHSP: %s\n", err)
						}
					}
					word = mem_read(temp)

				}

			case TRAP_HALT:
				fmt.Println("HALT")
				running = false
			}

		}

	}

	select {
	case <-ctx.Done():
		//log.Printf("exiting!!!!!")
		restore_input_buffering()
	}

}

func swap16(x uint16) uint16 {
	return (x << 8) | (x >> 8)
}

func update_flags(r uint16) {
	if register[r] == 0 {
		register[COND] = flags.ZRO
	} else if register[r]>>15 != 0 {
		register[COND] = flags.NEG
	} else {
		register[COND] = flags.POS
	}
	// register[PSR] &= 0xFFF8
	// if register[r] == 0 {
	// 	register[PSR] |= flags.ZRO
	// } else if register[r]>>15 == 1 {
	// 	register[PSR] |= flags.NEG
	// } else {
	// 	register[PSR] |= flags.POS
	// }
}

// sign extend
func sext(x, bit_count uint16) uint16 {
	if ((x >> (bit_count - 1)) & 0b1) != 0 {
		x |= (0xFFFF << bit_count)
	}
	return x

}

func mem_read(addr uint16) uint16 {

	if addr == memory_mapped_register.KBDR {
		memory[memory_mapped_register.KBSR] &= 0x7FFF
	}
	//log.Printf("mem_read-> value:%04x at address:%04x", memory[addr], addr)
	return memory[addr]
}

func process_keyboard() {
	if (mem_read(memory_mapped_register.KBSR)&0x8000 == 0) && (len(keyBuffer) > 0) {
		memory[memory_mapped_register.KBSR] |= 0x8000
		memory[memory_mapped_register.KBDR] = uint16(<-keyBuffer)

	}
}

// func check_key() (int, error) {
// 	var readFds unix.FdSet
// 	readFds.Zero()
// 	readFds.Set(int(os.Stdin.Fd()))

// 	var timeout unix.Timeval
// 	return unix.Select(1, &readFds, nil, nil, &timeout)
// }

func read_program(file_name string) error {
	//  //log.Println(file_name)
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
	origin := int(swap16(uint16((*file)[1]) | uint16((*file)[0])))

	max_read := User_Space_Start - saved_stack_pointer_register.USP
	//log.Println(origin, (max_read), len(*file))
	i := 0
	for j := 2; j < min(int(max_read), len(*file)); j += 2 {
		//  //log.Printf("address:%04x value:%04x", origin+i, uint16((*file)[j])<<8|uint16((*file)[j+1]))
		memory[origin+i] = uint16((*file)[j])<<8 | uint16((*file)[j+1])
		i++
	}
	// //log.Println(j+2)
	return nil

}

func disable_input_buffering() {
	termios.Tcgetattr(os.Stdin.Fd(), &orig_terminal_config)
	new_termios := orig_terminal_config
	//todo: add sigint flag
	new_termios.Lflag &^= unix.ICANON | unix.ECHO
	termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &new_termios)

}
func restore_input_buffering() {
	termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &orig_terminal_config)
}

func poll_keyboard() {
	ticker := time.NewTicker(5 * time.Millisecond)
	for range ticker.C {
		if mem_read(memory_mapped_register.KBSR)&0x8000 == 0 {
			buf := make([]byte, 1)
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				continue
			}

			for _, b := range buf[:n] {
				keyBuffer <- b
			}
		}
	}
}
