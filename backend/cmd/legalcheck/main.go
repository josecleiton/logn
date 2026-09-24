// legalcheck confere os documentos legais antes de virarem migração.
//
// Uso: go run ./cmd/legalcheck [--release] <pasta com <kind>.<locale>.html>
//
// Sem --release, confere a estrutura: os seis arquivos, tags e links permitidos, o
// `lang` de cada um e os mesmos ids de seção nas três línguas. Com --release, recusa
// também os marcadores de rascunho — é a porta para o texto final.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/josecleiton/logn/backend/internal/legal"
)

func main() {
	release := flag.Bool("release", false, "recusa marcadores de rascunho")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "uso: legalcheck [--release] <pasta>")
		os.Exit(2)
	}

	errs := legal.ValidateSet(flag.Arg(0), *release)
	for _, err := range errs {
		fmt.Fprintln(os.Stderr, "✗", err)
	}
	if len(errs) > 0 {
		os.Exit(1)
	}
	fmt.Println("documentos legais ok")
}
