package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Lesur-ai/dcmo5/internal/app"
	"github.com/Lesur-ai/dcmo5/internal/app/config"
	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
)

func main() {
	romPath := flag.String("rom", "", "chemin vers la ROM système MO5 (16 Ko)")
	tapePath := flag.String("tape", "", "fichier cassette .k7 à monter")
	diskPath := flag.String("disk", "", "fichier disquette .fd à monter")
	cartPath := flag.String("cart", "", "fichier cartouche .rom à monter")
	flag.Parse()

	// Charger les préférences pour fallback
	store, err := config.NewStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "dcmo5: config:", err)
		// Non fatal : on continue sans config
	}
	var cfg config.Config
	if store != nil {
		cfg, _ = store.Load()
	}

	// Résoudre les chemins : CLI prioritaire, puis config
	if *romPath == "" {
		*romPath = cfg.ROMPath
	}
	if *tapePath == "" {
		*tapePath = cfg.LastTape
	}
	if *diskPath == "" {
		*diskPath = cfg.LastDisk
	}
	if *cartPath == "" {
		*cartPath = cfg.LastCart
	}

	opts := core.Options{}
	romMissing := false
	// Descripteurs des médias ouverts au démarrage, confiés ensuite à l'App
	// pour fermeture propre en cas de remplacement via le menu.
	var tapeCloser, diskCloser io.Closer

	// ROM système
	if *romPath != "" {
		data, err := os.ReadFile(*romPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dcmo5: ROM:", err)
			os.Exit(1)
		}
		opts.ROMSys = data
	} else {
		romMissing = true
		fmt.Fprintln(os.Stderr, "dcmo5: ROM manquante — lancez avec -rom /chemin/mo5.rom")
		fmt.Fprintln(os.Stderr, "dcmo5: l'émulateur démarrera sans ROM (état indéfini)")
	}

	// Cassette
	if *tapePath != "" {
		tape, err := impl.OpenTape(*tapePath, false)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dcmo5: cassette:", err)
			os.Exit(1)
		}
		opts.Tape = tape
		tapeCloser = tape
	}

	// Disquette
	if *diskPath != "" {
		disk, err := impl.OpenDisk(*diskPath, false)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dcmo5: disquette:", err)
			os.Exit(1)
		}
		opts.Disk = disk
		diskCloser = disk
	}

	// Cartouche
	if *cartPath != "" {
		cart, err := impl.OpenCartridge(*cartPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dcmo5: cartouche:", err)
			os.Exit(1)
		}
		opts.Cartridge = cart
	}

	machine, err := core.NewMachine(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dcmo5: init machine:", err)
		os.Exit(1)
	}
	machine.Reset()

	// Sauvegarder uniquement le chemin ROM : les médias (tape/disk/cart) sont
	// acceptés en CLI et passés à core.Options, mais l'émulation I/O
	// (cassette, disque, cartouche) sera branchée dans le core en P6+.
	// On ne persiste pas les médias non encore fonctionnels pour ne pas
	// induire l'utilisateur en erreur.
	if store != nil && !romMissing {
		cfg.ROMPath = *romPath
		store.Save(cfg)
	}

	a := app.New(machine)
	a.SetROMStatus(romMissing)
	a.SetMediaNames(*romPath, *tapePath, *diskPath, *cartPath)
	a.SetStartupMediaClosers(tapeCloser, diskCloser)

	if err := app.Run(a); err != nil && !errors.Is(err, app.ErrUserQuit) {
		fmt.Fprintln(os.Stderr, "dcmo5:", err)
		os.Exit(1)
	}
}
