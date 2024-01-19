package vm

import (
	"fmt"
	"log"
)

type VM struct {
	memory memory
	cpu    cpu
}

func (vm *VM) Start(program *[]byte) {
	vm.cpu.io.enableRawMode()
	vm.readProgramFile(program)
	vm.cpu.start()
}

func (vm *VM) Stop(){
	vm.cpu.stop()
	vm.cpu.io.disableRawMode()
	vm.cpu.io.stdoutWriter.Write([]byte("\n"))
}

func NewVM() VM {
	mem := newMemory()
	return VM{
		cpu:    newCpu(&mem),
		memory: mem,
	}
}

func (vm *VM) readProgramFile(file *[]byte) error {
	if len(*file) < 4 {
		return fmt.Errorf("Error: File is too short.")
	}

	/* origin tells us where in memory to place the image
	most of the x86 systems are little endian, therefore keeping the byte sequence same as of the lc3 architecture(big endian)   */
	origin := int(uint16((*file)[0])<<8 | uint16((*file)[1]))
	max_read := MemoryMappedRegistersStart - UserSpaceStart
	log.Printf("Size: %0.2f KB", float32(len(*file))/1024)

	i := 0
	for j := 2; j < min(int(max_read), len(*file)); j += 2 {
		(*vm.memory.Ram)[origin+i] = word((*file)[j])<<8 | word((*file)[j+1])
		i++
	}

	return nil

}

