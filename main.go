package main

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

//opcodes
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
	NEG uint
}{
	1 << 0,
	1 << 1,
	1 << 2,
}



