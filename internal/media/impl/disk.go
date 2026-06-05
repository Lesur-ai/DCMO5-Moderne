package impl

import (
	"fmt"
	"os"

	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// FileDisk implémente media.Disk sur un fichier .fd.
// Format : spec.FDFaces × spec.FDTracks × spec.FDSectors × 256 octets = 327 680 o.
type FileDisk struct {
	f        *os.File
	readOnly bool
}

// OpenDisk ouvre un fichier disquette .fd existant.
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
	if info.Size() != int64(spec.FDDiskSize) {
		f.Close()
		return nil, fmt.Errorf("disk: taille %d invalide (attendu %d)", info.Size(), spec.FDDiskSize)
	}
	return &FileDisk{f: f, readOnly: readOnly}, nil
}

// NewDisk crée une disquette vierge (zéros) dans le fichier path.
func NewDisk(path string) (*FileDisk, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("disk: create: %w", err)
	}
	if err := f.Truncate(int64(spec.FDDiskSize)); err != nil {
		f.Close()
		return nil, fmt.Errorf("disk: truncate: %w", err)
	}
	return &FileDisk{f: f, readOnly: false}, nil
}

// Close ferme le fichier disquette.
func (d *FileDisk) Close() error { return d.f.Close() }

// sectorOffset calcule l'offset d'un secteur dans le fichier.
func sectorOffset(unit, track, sector int) (int64, error) {
	if unit < 0 || unit >= spec.FDFaces {
		return 0, fmt.Errorf("disk: face %d hors-bornes", unit)
	}
	if track < 0 || track >= spec.FDTracks {
		return 0, fmt.Errorf("disk: piste %d hors-bornes", track)
	}
	if sector < 0 || sector >= spec.FDSectors {
		return 0, fmt.Errorf("disk: secteur %d hors-bornes", sector)
	}
	off := int64(unit*spec.FDTracks*spec.FDSectors+track*spec.FDSectors+sector) * spec.FDSectorSize
	return off, nil
}

func (d *FileDisk) ReadSector(unit, track, sector int) ([256]byte, error) {
	off, err := sectorOffset(unit, track, sector)
	if err != nil {
		return [256]byte{}, err
	}
	var buf [256]byte
	if _, err := d.f.ReadAt(buf[:], off); err != nil {
		return [256]byte{}, fmt.Errorf("disk: read sector: %w", err)
	}
	return buf, nil
}

func (d *FileDisk) WriteSector(unit, track, sector int, data [256]byte) error {
	if d.readOnly {
		return fmt.Errorf("disk: lecture seule")
	}
	off, err := sectorOffset(unit, track, sector)
	if err != nil {
		return err
	}
	if _, err := d.f.WriteAt(data[:], off); err != nil {
		return fmt.Errorf("disk: write sector: %w", err)
	}
	return nil
}

func (d *FileDisk) FormatUnit(unit int) error {
	if d.readOnly {
		return fmt.Errorf("disk: lecture seule")
	}
	if unit < 0 || unit >= spec.FDFaces {
		return fmt.Errorf("disk: face %d hors-bornes", unit)
	}
	zeros := make([]byte, spec.FDTracks*spec.FDSectors*spec.FDSectorSize)
	off := int64(unit) * int64(spec.FDTracks*spec.FDSectors*spec.FDSectorSize)
	if _, err := d.f.WriteAt(zeros, off); err != nil {
		return fmt.Errorf("disk: format: %w", err)
	}
	return nil
}
