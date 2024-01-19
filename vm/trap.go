package vm

const (
	TRAP_GETC  word = 0x20 /* get character from keyboard, not echoed onto the terminal */
	TRAP_OUT   word = 0x21 /* output a character */
	TRAP_PUTS  word = 0x22 /* output a word string */
	TRAP_IN    word = 0x23 /* get character from keyboard, echoed onto the terminal */
	TRAP_PUTSP word = 0x24 /* output a byte string */
	TRAP_HALT  word = 0x25 /* halt the program */
)

