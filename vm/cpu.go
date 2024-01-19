package vm

import (
	"fmt"
	"log"
)

type word uint16

type cpu_flag uint16

// general purpose registers
const (
	R0 = 0b000
	R1 = 0b001
	R2 = 0b010
	R3 = 0b011
	R4 = 0b100
	R5 = 0b101
	R6 = 0b110
	R7 = 0b111
)

// flags
const (
	FLAG_POS cpu_flag = 0b001
	FLAG_ZRO          = 0b010
	FLAG_NEG          = 0b100
)

// memory mapped register addresses
const (
	KBSR = MemoryMappedRegistersStart          /* keyboard status register */
	KBDR = MemoryMappedRegistersStart + 0x0002 /* keyboard data register */
)

// opcodes
const (
	OP_BR word = iota
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

type cpu struct {
	running           bool
	memory            memory
	internalRegisters struct {
		pc,
		count word
		cond cpu_flag
	}
	generalPurposeRegisters [8]word
	io                      io
}

func (cpu *cpu) start() {
	go cpu.io.pollKeyboard(cpu.memRead)
	cpu.running = true

	for cpu.running {
		cpu.io.processKeyboard(cpu.memory.read, cpu.memory.write)
		instruction := cpu.memRead(cpu.internalRegisters.pc)
		cpu.internalRegisters.pc++
		cpu.decodeAndExecuteInstruction(instruction)

	}
}

func (cpu *cpu) stop() {
	cpu.running = false
}

func (cpu *cpu) decodeAndExecuteInstruction(instruction word) {
	op := instruction >> 12

	switch op {
	case OP_ADD:
		dr := (instruction >> 9) & 0b111
		sr1 := (instruction >> 6) & 0b111
		imm_flag := (instruction >> 5) & 0b1

		if imm_flag == 1 {
			imm5 := instruction & 0x1F
			log.Printf("0x%04x ADD: dr=%03b sr1=%03b imm5=0x%02x", cpu.internalRegisters.pc, dr, sr1, imm5)
			cpu.generalPurposeRegisters[dr] = cpu.generalPurposeRegisters[sr1] + sext(imm5, 5)
		} else {
			sr2 := (instruction) & 0b111
			log.Printf("0x%04x ADD: dr=%03b sr1=%03b r1=%03b", cpu.internalRegisters.pc, dr, sr1, sr2)
			cpu.generalPurposeRegisters[dr] = cpu.generalPurposeRegisters[sr1] + cpu.generalPurposeRegisters[sr2]
		}

		cpu.updateFlags(dr)

	case OP_AND:

		dr := (instruction >> 9) & 0b111
		sr1 := (instruction >> 6) & 0b111
		imm_flag := (instruction >> 5) & 0b1

		if imm_flag == 1 {
			imm5 := instruction & 0b11111
			log.Printf("0x%04x AND: dr=%03b sr1=%03b imm5=0x%02x", cpu.internalRegisters.pc, dr, sr1, imm5)
			cpu.generalPurposeRegisters[dr] = cpu.generalPurposeRegisters[sr1] & sext(imm5, 5)
		} else {
			sr2 := (instruction) & 0b111
			log.Printf("0x%04x AND: dr=%03b sr1=%03b r1=%03b", cpu.internalRegisters.pc, dr, sr1, sr2)
			cpu.generalPurposeRegisters[dr] = cpu.generalPurposeRegisters[sr1] & cpu.generalPurposeRegisters[sr2]
		}

		cpu.updateFlags(dr)

	case OP_NOT:

		dr := (instruction >> 9) & 0b111
		sr := (instruction >> 6) & 0b111

		log.Printf("0x%04x NOT: dr=%03b sr=%03b", cpu.internalRegisters.pc, dr, sr)

		cpu.generalPurposeRegisters[dr] = ^cpu.generalPurposeRegisters[sr]
		cpu.updateFlags(dr)

	case OP_BR:

		nzp := (instruction >> 9) & 0b111
		pcoffset9 := instruction & 0x1FF
		cond := word(cpu.internalRegisters.cond)

		log.Printf("0x%04x BR: nzp=%03b pcoffset9=0x%03x", cpu.internalRegisters.pc, nzp, pcoffset9)

		if (nzp & cond) != 0 {
			cpu.internalRegisters.pc += sext(pcoffset9, 9)
		}

	case OP_JMP:
		br := (instruction >> 6) & 0b111

		log.Printf("0x%04x JMP: br=%03b", cpu.internalRegisters.pc, br)

		cpu.internalRegisters.pc = cpu.generalPurposeRegisters[br]

	case OP_JSR:

		cpu.generalPurposeRegisters[R7] = cpu.internalRegisters.pc

		bit11 := (instruction >> 11) & 0b1
		if bit11 == 1 {
			pcoffset11 := instruction & 0x7FF
			log.Printf("0x%04x JSR: pcoffset11=0x%03x", cpu.internalRegisters.pc, pcoffset11)
			cpu.internalRegisters.pc += sext(pcoffset11, 11)
		} else {
			br := (instruction >> 6) & 0b111
			log.Printf("0x%04x JSRR: br=%03b", cpu.internalRegisters.pc, br)
			cpu.internalRegisters.pc = cpu.generalPurposeRegisters[br]
		}
	case OP_LD:

		dr := (instruction >> 9) & 0b111
		pcoffset9 := instruction & 0x1FF

		log.Printf("0x%04x LD: dr=%03b pcoffset9=0x%03x", cpu.internalRegisters.pc, dr, pcoffset9)

		cpu.generalPurposeRegisters[dr] = cpu.memRead(cpu.internalRegisters.pc + sext(pcoffset9, 9))
		cpu.updateFlags(dr)

	case OP_LDI:

		dr := (instruction >> 9) & 0b111
		pcoffset9 := instruction & 0x1FF

		log.Printf("0x%04x LDI: dr=%03b pcoffset9=0x%03x", cpu.internalRegisters.pc, dr, pcoffset9)
		cpu.generalPurposeRegisters[dr] = cpu.memRead(cpu.memRead(cpu.internalRegisters.pc + sext(pcoffset9, 9)))
		cpu.updateFlags(dr)

	case OP_LDR:

		dr := (instruction >> 9) & 0b111
		br := (instruction >> 6) & 0b111
		pcoffset6 := instruction & 0x3F

		log.Printf("0x%04x LDR: dr=%03b dr=%03b pcoffset6=0x%02x", cpu.internalRegisters.pc, dr, br, pcoffset6)

		cpu.generalPurposeRegisters[dr] = cpu.memRead(cpu.generalPurposeRegisters[br] + sext(pcoffset6, 6))
		cpu.updateFlags(dr)

	case OP_LEA:

		dr := (instruction >> 9) & 0b111
		pcoffset9 := instruction & 0x1FF

		log.Printf("0x%04x LEA: dr=%03b pcoffset9=0x%03x", cpu.internalRegisters.pc, dr, pcoffset9)

		cpu.generalPurposeRegisters[dr] = cpu.internalRegisters.pc + sext(pcoffset9, 9)
		cpu.updateFlags(dr)

	case OP_RTI:
		log.Printf("0x%04x RTI: unimplemented opcode", cpu.internalRegisters.pc)

	case OP_ST:
		sr := (instruction >> 9) & 0b111
		pcoffset9 := instruction & 0x1FF

		log.Printf("0x%04x ST: sr=%03b pcoffset9=0x%03x", cpu.internalRegisters.pc, sr, pcoffset9)

		computed_address := cpu.internalRegisters.pc + sext(pcoffset9, 9)
		cpu.memWrite(computed_address, cpu.generalPurposeRegisters[sr])

	case OP_STI:
		sr := (instruction >> 9) & 0b111
		pcoffset9 := instruction & 0x1FF

		log.Printf("0x%04x STI: sr=%03b pcoffset9=0x%03x", cpu.internalRegisters.pc, sr, pcoffset9)

		cpu.memWrite(cpu.memRead(cpu.internalRegisters.pc+sext(pcoffset9, 9)), cpu.generalPurposeRegisters[sr])

	case OP_STR:
		sr := (instruction >> 9) & 0b111
		br := (instruction >> 6) & 0b111
		pcoffset6 := instruction & 0x3F

		log.Printf("0x%04x STR: sr=%03b br=%03b pcoffset6=0x%02x", cpu.internalRegisters.pc, sr, br, pcoffset6)

		computed_address := cpu.generalPurposeRegisters[br] + sext(pcoffset6, 6)

		cpu.memWrite(computed_address, cpu.generalPurposeRegisters[sr])

	case OP_TRAP:
		log.Printf("0x%04x TRAP: 0x%02x", cpu.internalRegisters.pc, instruction&0xFF)

		switch instruction & 0xFF {
		case TRAP_GETC:
			cpu.generalPurposeRegisters[R0] = word(<-cpu.io.keyBuffer)
			cpu.updateFlags(R0)

		case TRAP_OUT:
			c := byte(cpu.generalPurposeRegisters[R0])
			cpu.io.stdoutWriter.Write([]byte{c})

		case TRAP_PUTS:
			temp := cpu.generalPurposeRegisters[R0]

			for c := cpu.memRead(temp) & 0xFFFF; c != 0; {
				cpu.io.stdoutWriter.Write([]byte{byte(c)})
				temp++
				c = cpu.memRead(temp) & 0xFFFF
			}

		case TRAP_IN:
			cpu.io.stdoutWriter.Write([]byte("Enter a character: "))

			c := <-cpu.io.keyBuffer

			cpu.io.stdoutWriter.Write([]byte{c})

			cpu.generalPurposeRegisters[R0] = word(c)
			cpu.updateFlags(R0)

		case TRAP_PUTSP:
			temp := cpu.generalPurposeRegisters[R0]

			for word := cpu.memRead(temp); word != 0; temp++ {
				cpu.io.stdoutWriter.Write([]byte{byte(word)})

				if word>>8 != 0 {
					cpu.io.stdoutWriter.Write([]byte{byte(word >> 8)})

				}
				word = cpu.memRead(temp)

			}

		case TRAP_HALT:
			fmt.Println("HALT")
			cpu.stop()
		}
	}
}

func (cpu *cpu) memWrite(addr, value word) {
	if addr == KBSR || addr == KBDR {
		return
	}
	cpu.memory.write(addr, value)
}

func (cpu *cpu) memRead(addr word) word {

	if addr == KBDR {
		cpu.memory.write(KBSR, cpu.memory.read(KBSR)&0x7FFF)
	}
	return cpu.memory.read(addr)
}

func (cpu *cpu) updateFlags(r word) {
	if cpu.generalPurposeRegisters[r] == 0 {
		cpu.internalRegisters.cond = FLAG_ZRO
	} else if cpu.generalPurposeRegisters[r]>>15 != 0 {
		cpu.internalRegisters.cond = FLAG_NEG
	} else {
		cpu.internalRegisters.cond = FLAG_POS
	}
}

func sext(x, bit_count word) word {
	if ((x >> (bit_count - 1)) & 0b1) != 0 {
		x |= (0xFFFF << bit_count)
	}
	return x

}
func newCpu(memory *memory) cpu {
	return cpu{
		running: false,
		memory:  *memory,
		internalRegisters: struct {
			pc,
			count word
			cond cpu_flag
		}{pc: UserSpaceStart, cond: FLAG_ZRO},
		io: newIO(),
	}
}
