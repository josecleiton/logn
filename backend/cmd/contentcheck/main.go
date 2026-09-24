// contentcheck confere os arquivos da trilha antes de virarem migração.
//
// Uso: go run ./cmd/contentcheck [--release] <pasta com trilha/ e glossario/>
//
// Erro de forma — payload que o banco recusaria, texto vazio, opção de TAG fora do
// glossário — reprova sempre. O que ainda falta fazer — língua sem tradução, código com
// identificador em português — aparece como pendência, e só reprova com --release, que
// é a porta para a submissão às lojas.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/josecleiton/logn/backend/internal/content"
)

func main() {
	release := flag.Bool("release", false, "pendência também reprova")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "uso: contentcheck [--release] <pasta>")
		os.Exit(2)
	}

	var errs, pending int
	for _, f := range content.Validate(flag.Arg(0)) {
		switch {
		case !f.Pending:
			errs++
			fmt.Fprintln(os.Stderr, "✗", f)
		case *release:
			errs++
			fmt.Fprintln(os.Stderr, "✗ (pendente)", f)
		default:
			pending++
			fmt.Fprintln(os.Stderr, "·", f)
		}
	}
	if errs > 0 {
		fmt.Fprintf(os.Stderr, "%d erro(s)\n", errs)
		os.Exit(1)
	}
	if pending > 0 {
		fmt.Printf("conteúdo ok, com %d pendência(s) que --release recusaria\n", pending)
		return
	}
	fmt.Println("conteúdo ok")
}
