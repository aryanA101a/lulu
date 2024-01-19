package main

import (
	"context"
	"flag"

	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	"github.com/aryanA101a/lulu/vm"
	"golang.org/x/sys/unix"
)



func main() {

	logfile:=handle_commandline_args()
	if logfile!=nil{
		defer logfile.Close()
	}
	inPath := flag.Args()[0]
	program, err := read_program(inPath)
	if err != nil {
		fmt.Printf("Error opening file(%s): %v\n", inPath, err)
		os.Exit(2)
	}

	vm := vm.NewVM()
	
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, unix.SIGINT, unix.SIGTSTP)
	defer stop()

	go exit(ctx,vm.Stop)

	vm.Start(program)


}

func handle_commandline_args() *os.File{
	var (
		f *os.File
		verboseFlag,
		helpFlag bool
		logfile string
	)
	flag.BoolVar(&verboseFlag, "v", false, "logging")
	flag.BoolVar(&helpFlag, "h", false, "help")
	flag.StringVar(&logfile, "logfile", "", "path to logfile")


	flag.Parse()
	if helpFlag || flag.NArg() != 1 {
		flag.Usage()
		os.Exit(0)
	}

	if verboseFlag {
		if logfile != "" {
			var err error
			f, err = os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				log.Printf("Error opening logfile: %v\n", err)
				f.Close()
				os.Exit(2)
			}
			log.SetOutput(f)
		}

	} else {
		log.SetOutput(io.Discard)
	}
	return f
}

func read_program(file_name string) (*[]byte, error) {
	log.Printf("Loading: %s", file_name)
	file, err := os.ReadFile(file_name)
	if err != nil {
		return nil, err
	}

	return &file, nil
}


func exit(ctx context.Context,stopVM func()) {
	<-ctx.Done()
	stopVM()
	log.Println("exiting!!")
	os.Exit(0)

}