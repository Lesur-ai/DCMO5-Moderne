package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Lesur-ai/dcmo5/internal/app"
	"github.com/Lesur-ai/dcmo5/internal/app/config"
	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/machine/mo5"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
)

// version est la version du binaire, injectée à la compilation via
// -ldflags="-X main.version=<tag>" (cf. .github/workflows/release.yml).
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "afficher la version et quitter")
	machineID := flag.String("machine", "mo5", "machine à émuler (défaut: mo5)")
	romPath := flag.String("rom", "", "chemin vers la ROM système MO5 (16 Ko)")
	tapePath := flag.String("tape", "", "fichier cassette .k7 à monter")
	diskPath := flag.String("disk", "", "fichier disquette .fd à monter")
	cartPath := flag.String("cart", "", "fichier cartouche .rom à monter")
	diskRomPath := flag.String("disk-rom", "", "ROM du contrôleur de disquette CD90-640 (~2 Ko ; auto-détectée à côté de la ROM système si absente)")
	noAudio := flag.Bool("no-audio", false, "désactiver la sortie audio")
	execSeq := flag.String("exec", "", "séquence de touches tapée au démarrage (\\n = ENTRÉE), ex: '10 CLS\\nRUN\\n'")
	execDelay := flag.Float64("exec-delay", 3, "délai en secondes avant de taper --exec (le temps que l'invite BASIC apparaisse)")
	flag.Parse()

	if *showVersion {
		fmt.Println("dcmo5", version)
		return
	}

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

	opts := core.Options{
		// Aligne les vraies ROM MO5 sur le modèle trap, comme dcmo5 v11 : ROM
		// système (cassette/crayon/imprimante) et ROM contrôleur de disquette
		// CD90-640 (lire/écrire/formater + amorçage DOS). Patch en mémoire ;
		// fichiers ROM intacts.
		PatchSystemROM: true,
		// Remonte les erreurs d'E/S MO5 (équiv. boîte Erreur(n) réf C) sur stderr.
		OnError: func(code int) {
			fmt.Fprintf(os.Stderr, "dcmo5: erreur E/S MO5 %d (%s)\n", code, core.IOErrorLabel(code))
		},
	}
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

	// ROM du contrôleur de disquette CD90-640 : flag explicite, sinon auto-détection
	// d'un « cd90-640.rom » à côté de la ROM système. Indispensable pour la disquette.
	dcRomPath := *diskRomPath
	if dcRomPath == "" && *romPath != "" {
		candidate := filepath.Join(filepath.Dir(*romPath), "cd90-640.rom")
		if _, err := os.Stat(candidate); err == nil {
			dcRomPath = candidate
		}
	}
	if dcRomPath != "" {
		if data, err := os.ReadFile(dcRomPath); err != nil {
			fmt.Fprintln(os.Stderr, "dcmo5: ROM contrôleur disquette:", err)
		} else {
			opts.DiskControllerROM = data
		}
	} else if *diskPath != "" {
		fmt.Fprintln(os.Stderr, "dcmo5: disquette montée sans ROM contrôleur CD90-640 "+
			"(-disk-rom) — le DOS sera inopérant")
	}

	// Construction de la machine sélectionnée via le registre des profils. Le MO5
	// est bâti par la voie cœur (pour brancher l'instrumentation E/S non couverte par
	// le contrat) puis enrobé par l'adaptateur. Les autres profils seront constructibles
	// en CLI au fil des lots v2 ; le launcher (lot 11) les instanciera génériquement.
	var m machine.Machine
	switch *machineID {
	case "mo5":
		coreM, err := core.NewMachine(opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dcmo5: init machine:", err)
			os.Exit(1)
		}
		// Instrumentation E/S optionnelle (diagnostic). Même politique que profile.New
		// (option A) : le writer est résolu par mo5.IOTraceWriter, source unique du
		// gating env (DCMO5_IO_TRACE / DCMO5_IO_TRACE_FILE).
		if traceW := mo5.IOTraceWriter(); traceW != nil {
			coreM.EnableIOTrace(traceW)
		}
		coreM.Reset()
		m = mo5.Wrap(coreM)
	default:
		ids := make([]string, 0)
		for _, p := range machine.Profiles() {
			ids = append(ids, p.ID)
		}
		fmt.Fprintf(os.Stderr, "dcmo5: machine inconnue %q. Disponibles : %s\n", *machineID, strings.Join(ids, ", "))
		os.Exit(1)
	}

	// Sauvegarder uniquement le chemin ROM : les médias (tape/disk/cart) sont
	// acceptés en CLI et passés à core.Options, mais l'émulation I/O
	// (cassette, disque, cartouche) sera branchée dans le core en P6+.
	// On ne persiste pas les médias non encore fonctionnels pour ne pas
	// induire l'utilisateur en erreur.
	if store != nil && !romMissing {
		cfg.ROMPath = *romPath
		store.Save(cfg)
	}

	a := app.New(m)
	a.SetROMStatus(romMissing)
	a.SetMediaNames(*romPath, *tapePath, *diskPath, *cartPath)
	a.SetStartupMediaClosers(tapeCloser, diskCloser)
	if *noAudio {
		a.DisableAudio()
	}
	if *execSeq != "" {
		// Le shell passe « \n » littéral (deux caractères) ; on le convertit en
		// vrai retour-chariot (de même pour \t). Les guillemets du programme
		// BASIC sont préservés (pas d'unquote global).
		seq := strings.ReplaceAll(*execSeq, `\n`, "\n")
		seq = strings.ReplaceAll(seq, `\t`, "\t")
		a.SetExec(seq, *execDelay)
	}

	if err := app.Run(a); err != nil && !errors.Is(err, app.ErrUserQuit) {
		fmt.Fprintln(os.Stderr, "dcmo5:", err)
		os.Exit(1)
	}
}
