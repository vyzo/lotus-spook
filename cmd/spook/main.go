package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("spook")

func init() {
	logging.SetLogLevel("spook", "DEBUG")
}

func main() {
	npeers := flag.Int("n", 1, "Number of embedded peers")
	quiet := flag.Bool("q", false, "Only log errors")
	file := flag.String("f", "", "Output file; use Stdout if omitted")
	idPath := flag.String("id", "", "permanent identity file")
	userBootstrappers := flag.String("b", "", "coma separated list of bootstrappers")
	flag.Parse()

	if *quiet {
		logging.SetLogLevel("*", "ERROR")
	}

	if *userBootstrappers != "" {
		bootstrappers = strings.Split(*userBootstrappers, ",")
	}

	var w io.Writer
	if *file == "" {
		w = os.Stdout
	} else {
		f, err := os.OpenFile(*file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Errorf("error opening output file: %s", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}

	logger := NewLogger(w)

	var wg sync.WaitGroup
	for i := 0; i < *npeers; i++ {
		wg.Add(1)
		var idFile string
		if *idPath != "" {
			idFile = fmt.Sprintf("%s.%d", *idPath, i)
		}
		n, err := NewNode(logger, idFile)
		if err != nil {
			log.Errorf("error creating peer: %s", err)
			os.Exit(1)
		}
		go n.Background(&wg)
	}

	wg.Wait()
}
