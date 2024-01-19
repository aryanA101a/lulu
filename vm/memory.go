package vm


const MemorySize = 1 << 16
const (
	Trap_Vector_Table_Start    = 0x0000
	InterruptVectorTableStart  = 0x0100
	SystemSpaceStart           = 0x0200
	UserSpaceStart             = 0x3000
	MemoryMappedRegistersStart = 0xFE00
)

type memory struct {
	Ram *[]word
}

func (mem *memory) write(addr, value word) {
if addr==KBDR{
}
	(*mem.Ram)[addr] = value
}

func (mem memory) read(addr word) word {
	return (*mem.Ram)[addr]
}

func newMemory() memory {
	ram:=make([]word,MemorySize)
	return memory{Ram: &ram}
}
