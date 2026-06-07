package impl

import (
	"fmt"
	"os"

	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// FileDisk implémente media.Disk sur un fichier .fd.
//
// La géométrie est celle du contrôleur CD90-640 : secteurs de 256 o, 16 secteurs
// par piste, stride d'unité de 1280 secteurs (80 pistes). La densité réelle
// (simple/double face, 40/80 pistes) découle de la TAILLE du fichier : un accès
// secteur est borné dynamiquement par la taille réelle, jamais par une constante.
// On accepte donc aussi bien un .fd de 163 840 o (40 pistes, 1 face) que de
// 327 680 o (80 pistes ou 2 faces). Ref C: dcmo5devices.c Readsector().
type FileDisk struct {
	f        *os.File
	size     int64
	readOnly bool
}

// OpenDisk ouvre un fichier disquette .fd existant. Toute taille multiple de la
// taille de secteur (et non vide) est acceptée — la densité est déduite de la
// taille. Le contrôleur borne les accès par cette taille réelle.
func OpenDisk(path string, readOnly bool) (*FileDisk, error) {
	flag := os.O_RDWR
	if readOnly {
		flag = os.O_RDONLY
	}
	f, err := os.OpenFile(path, flag, 0)
	if err != nil {
		return nil, fmt.Errorf("disk: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("disk: stat: %w", err)
	}
	size := info.Size()
	if size <= 0 || size%int64(spec.FDSectorSize) != 0 {
		f.Close()
		return nil, fmt.Errorf("disk: taille %d invalide (doit être un multiple non nul de %d)", size, spec.FDSectorSize)
	}
	return &FileDisk{f: f, size: size, readOnly: readOnly}, nil
}

// NewDisk crée une disquette vierge (spec.FDDiskSize, soit une unité de 80 pistes).
func NewDisk(path string) (*FileDisk, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("disk: create: %w", err)
	}
	if err := f.Truncate(int64(spec.FDDiskSize)); err != nil {
		f.Close()
		return nil, fmt.Errorf("disk: truncate: %w", err)
	}
	return &FileDisk{f: f, size: int64(spec.FDDiskSize), readOnly: false}, nil
}

// Close ferme le fichier disquette.
func (d *FileDisk) Close() error { return d.f.Close() }

// sectorOffset calcule l'offset d'un secteur dans le fichier et le borne par la
// taille réelle de la disquette (densité variable). Ref C: dcmo5devices.c —
// u≤3, p(0x204a)==0, p(0x204b)≤79, s∈[1,16] ; s += 16*p + 1280*u ;
// si (s<<8) > taille_fichier → erreur ; offset = (s-1)<<8.
func (d *FileDisk) sectorOffset(unit, track, sector int) (int64, error) {
	if unit < 0 || unit >= spec.FDMaxUnits {
		return 0, fmt.Errorf("disk: unité %d hors-bornes [0,%d)", unit, spec.FDMaxUnits)
	}
	if track < 0 || track >= spec.FDTracksPerUnit {
		return 0, fmt.Errorf("disk: piste %d hors-bornes [0,%d)", track, spec.FDTracksPerUnit)
	}
	if sector < 1 || sector > spec.FDSectorsPerTrack {
		return 0, fmt.Errorf("disk: secteur %d hors-bornes [1,%d]", sector, spec.FDSectorsPerTrack)
	}
	s := sector + spec.FDSectorsPerTrack*track + spec.FDSectorsPerUnit*unit
	off := int64(s-1) * int64(spec.FDSectorSize)
	// Bornage dynamique : le secteur doit tenir entièrement dans le fichier réel.
	if off+int64(spec.FDSectorSize) > d.size {
		return 0, fmt.Errorf("disk: secteur (u%d p%d s%d) hors capacité du fichier (%d o)", unit, track, sector, d.size)
	}
	return off, nil
}

func (d *FileDisk) ReadSector(unit, track, sector int) ([256]byte, error) {
	// Pré-remplissage 0xE5 (motif « non formaté »), comme la réf C avant fread.
	buf := [256]byte{}
	for i := range buf {
		buf[i] = 0xE5
	}
	off, err := d.sectorOffset(unit, track, sector)
	if err != nil {
		return buf, err
	}
	if _, err := d.f.ReadAt(buf[:], off); err != nil {
		return buf, fmt.Errorf("disk: read sector: %w", err)
	}
	return buf, nil
}

func (d *FileDisk) WriteSector(unit, track, sector int, data [256]byte) error {
	if d.readOnly {
		return media.ErrWriteProtected
	}
	off, err := d.sectorOffset(unit, track, sector)
	if err != nil {
		return err
	}
	if _, err := d.f.WriteAt(data[:], off); err != nil {
		return fmt.Errorf("disk: write sector: %w", err)
	}
	return nil
}

// FormatUnit initialise une unité comme la réf C (dcmo5devices.c Formatdisk) :
// 40 pistes (640 secteurs) à 0xE5, piste 20 à 0xFF, puis FAT. Les écritures sont
// bornées par la taille réelle de la disquette.
func (d *FileDisk) FormatUnit(unit int) error {
	if d.readOnly {
		return media.ErrWriteProtected
	}
	if unit < 0 || unit >= spec.FDMaxUnits {
		return fmt.Errorf("disk: unité %d hors-bornes", unit)
	}
	base := int64(unit) * int64(spec.FDSectorsPerUnit) * int64(spec.FDSectorSize)

	writeAt := func(off int64, b []byte) error {
		// Hors capacité du fichier = échec de format (l'unité n'existe pas sur
		// cette densité). Ref C : un fwrite raté déclenche Diskerror(53).
		if off < 0 || off+int64(len(b)) > d.size {
			return fmt.Errorf("disk: format hors capacité (offset %d, taille %d)", off, d.size)
		}
		_, err := d.f.WriteAt(b, off)
		return err
	}

	// 640 secteurs (40 pistes) à 0xE5.
	e5 := make([]byte, spec.FDSectorSize)
	for i := range e5 {
		e5[i] = 0xE5
	}
	const fatTracks = 40
	for i := 0; i < fatTracks*spec.FDSectorsPerTrack; i++ {
		if err := writeAt(base+int64(i)*int64(spec.FDSectorSize), e5); err != nil {
			return fmt.Errorf("disk: format: %w", err)
		}
	}
	// Piste 20 (offset 0x14000) : 16 secteurs à 0xFF.
	ff := make([]byte, spec.FDSectorSize)
	for i := range ff {
		ff[i] = 0xFF
	}
	for i := 0; i < spec.FDSectorsPerTrack; i++ {
		if err := writeAt(base+0x14000+int64(i)*int64(spec.FDSectorSize), ff); err != nil {
			return fmt.Errorf("disk: format: %w", err)
		}
	}
	// FAT (offset 0x14100). Ref C : le buffer est ENCORE rempli de 0xFF (réutilisé
	// après la piste 20), puis buffer[0]=0 ; [0x29]=[0x2a]=0xFE ; [81..255]=0xFE.
	// Les octets 1..40 et 43..80 restent donc à 0xFF (pistes libres).
	fat := make([]byte, spec.FDSectorSize)
	for i := range fat {
		fat[i] = 0xFF
	}
	fat[0x00] = 0x00
	fat[0x29] = 0xFE
	fat[0x2A] = 0xFE
	for i := 81; i < spec.FDSectorSize; i++ {
		fat[i] = 0xFE
	}
	if err := writeAt(base+0x14100, fat); err != nil {
		return fmt.Errorf("disk: format FAT: %w", err)
	}
	return nil
}
