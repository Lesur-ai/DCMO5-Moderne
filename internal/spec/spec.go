// Package spec centralise les constantes matérielles TRANSVERSES des machines
// Thomson émulées : horloge CPU, vecteurs 6809, taux d'échantillonnage audio par
// défaut — et, en attendant leur généralisation par machine, le clavier MO5 et la
// géométrie des médias (cassette, disquette CD90-640).
//
// Les constantes propres au rendu et à la carte mémoire d'une machine donnée
// vivent dans son package (ex: internal/core pour le MO5 — géométrie écran,
// palette, carte mémoire : voir internal/core/mo5hw.go). Cf.
// DESIGN/MACHINE_PROFILES.md §« spec ne garde que le transverse ».
package spec

// Horloge CPU
const (
	CPUClockHz = 1_000_000 // Motorola 6809 à 1 MHz nominal
)

// Timing vidéo Thomson commun aux machines déjà portées.
const (
	VideoCyclesPerLine  = 64
	VideoLinesPerFrame  = 312
	VideoCyclesPerFrame = VideoCyclesPerLine * VideoLinesPerFrame
)

// Audio — taux d'échantillonnage par défaut (Hz). 48000 = taux natif des
// périphériques modernes, qui évite le rééchantillonnage du backend (source
// d'artefacts). Configurable par machine via core.Options.AudioSampleRate ;
// 22050 reste utilisable pour la fidélité/tests. Le niveau audio (résolution du
// haut-parleur) est propre à chaque machine (MO5 : core.AudioLevelMax).
const (
	AudioSampleRate = 48000
)

// Vecteurs 6809 (big-endian, au sommet de la mémoire) — transverses à toute
// machine bâtie sur le Motorola 6809.
const (
	VectorReset uint16 = 0xFFFE
	VectorNMI   uint16 = 0xFFFC
	VectorSWI   uint16 = 0xFFFA
	VectorIRQ   uint16 = 0xFFF8
	VectorFIRQ  uint16 = 0xFFF6
	VectorSWI2  uint16 = 0xFFF4
	VectorSWI3  uint16 = 0xFFF2
)

// Paramètres cassette .k7
const (
	K7BaudRate = 1200 // débit nominal cassette (bauds)
)

// Paramètres disquette .fd — géométrie du contrôleur CD90-640.
//
// Le contrôleur adresse une disquette par (unité, piste, secteur). La taille
// RÉELLE du fichier .fd détermine la densité (simple/double face, 40/80 pistes) :
// le décalage d'un secteur est borné dynamiquement par la taille du fichier,
// jamais par une taille figée. Ref C: dcmo5devices.c Readsector() —
// s += 16*p + 1280*u ; offset=(s-1)<<8 ; borné par ftell(ffd).
const (
	FDSectorSize      = 256                                 // octets par secteur
	FDSectorsPerTrack = 16                                  // secteurs par piste
	FDTracksPerUnit   = 80                                  // pistes adressables par unité (stride)
	FDMaxUnits        = 4                                   // unités 0..3
	FDSectorsPerUnit  = FDSectorsPerTrack * FDTracksPerUnit // 1280 secteurs/unité
	FDDiskSize        = FDSectorsPerUnit * FDSectorSize     // 327 680 o — taille par défaut d'une disquette créée
)
