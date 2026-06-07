package impl_test

// Tests de la densité variable du contrôleur CD90-640 : la taille réelle du .fd
// détermine la capacité, le décalage secteur est borné dynamiquement, et la
// géométrie (16 secteurs/piste, 1280 secteurs/unité) ne fait pas d'aliasing.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// makeFD crée un fichier .fd de size octets (rempli de zéros).
func makeFD(t *testing.T, name string, size int64) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	f.Close()
	return p
}

const (
	sizeSingle = 163840 // 40 pistes × 16 secteurs × 256 (1 face / simple densité)
	sizeDouble = 327680 // 80 pistes × 16 secteurs × 256 (double densité / 2 faces)
)

func TestDisk_AcceptsVariableSizes(t *testing.T) {
	for _, size := range []int64{sizeSingle, sizeDouble, int64(spec.FDSectorSize)} {
		d, err := impl.OpenDisk(makeFD(t, "ok.fd", size), false)
		if err != nil {
			t.Errorf("OpenDisk taille %d : erreur inattendue %v", size, err)
			continue
		}
		d.Close()
	}
	// Tailles invalides : vide, ou non multiple de la taille de secteur.
	for _, bad := range []int64{0, 100, spec.FDSectorSize + 1} {
		if _, err := impl.OpenDisk(makeFD(t, "bad.fd", bad), false); err == nil {
			t.Errorf("OpenDisk taille %d : erreur attendue", bad)
		}
	}
}

func TestDisk_DoubleDensity_Track79(t *testing.T) {
	// Disque double densité : la piste 79 (dernière) est accessible.
	dd, _ := impl.OpenDisk(makeFD(t, "dd.fd", sizeDouble), false)
	defer dd.Close()
	var data [256]byte
	for i := range data {
		data[i] = 0x5A
	}
	if err := dd.WriteSector(0, 79, 16, data); err != nil {
		t.Fatalf("double densité : écriture piste 79 secteur 16 refusée : %v", err)
	}
	got, err := dd.ReadSector(0, 79, 16)
	if err != nil || got != data {
		t.Errorf("double densité : relecture piste 79 KO (err=%v, égal=%v)", err, got == data)
	}

	// Disque simple densité : la piste 79 (et 40) dépasse la capacité → erreur.
	sd, _ := impl.OpenDisk(makeFD(t, "sd.fd", sizeSingle), false)
	defer sd.Close()
	if _, err := sd.ReadSector(0, 79, 1); err == nil {
		t.Error("simple densité : lecture piste 79 devrait échouer (hors capacité)")
	}
	if _, err := sd.ReadSector(0, 40, 1); err == nil {
		t.Error("simple densité : lecture piste 40 devrait échouer (hors capacité 40 pistes)")
	}
	// Mais la dernière piste valide (39) reste accessible.
	if _, err := sd.ReadSector(0, 39, 16); err != nil {
		t.Errorf("simple densité : piste 39 secteur 16 devrait être lisible : %v", err)
	}
}

func TestDisk_FormatStructure(t *testing.T) {
	// Vérifie la structure d'un format conforme à la réf C : 0xE5 sur les pistes
	// de données, 0xFF sur la piste 20, et la FAT (piste 20 secteur 2 = 0x14100).
	d, _ := impl.OpenDisk(makeFD(t, "fmt.fd", sizeDouble), false)
	defer d.Close()
	if err := d.FormatUnit(0); err != nil {
		t.Fatalf("FormatUnit: %v", err)
	}
	// Données : 0xE5.
	if got, _ := d.ReadSector(0, 0, 1); got[0] != 0xE5 {
		t.Errorf("piste 0 : 0x%02X, want 0xE5", got[0])
	}
	// Piste 20 secteur 1 (offset 0x14000) : 0xFF.
	if got, _ := d.ReadSector(0, 20, 1); got[0] != 0xFF || got[255] != 0xFF {
		t.Errorf("piste 20 s1 : [0]=0x%02X [255]=0x%02X, want 0xFF", got[0], got[255])
	}
	// FAT (piste 20 secteur 2, offset 0x14100) : [0]=0x00, pistes libres 0xFF, marqueurs 0xFE.
	fat, _ := d.ReadSector(0, 20, 2)
	checks := []struct {
		idx  int
		want byte
	}{{0x00, 0x00}, {0x01, 0xFF}, {0x28, 0xFF}, {0x29, 0xFE}, {0x2A, 0xFE}, {0x50, 0xFF}, {0x51, 0xFE}, {0xFF, 0xFE}}
	for _, c := range checks {
		if fat[c.idx] != c.want {
			t.Errorf("FAT[0x%02X] = 0x%02X, want 0x%02X", c.idx, fat[c.idx], c.want)
		}
	}
}

func TestDisk_FormatOutOfCapacity_Errors(t *testing.T) {
	// Formater l'unité 1 d'un disque à une seule unité (327680) doit échouer
	// (hors capacité), au lieu de réussir silencieusement.
	d, _ := impl.OpenDisk(makeFD(t, "cap.fd", sizeDouble), false)
	defer d.Close()
	if err := d.FormatUnit(1); err == nil {
		t.Error("FormatUnit(1) sur disque 1-unité : erreur attendue (hors capacité)")
	}
}

func TestDisk_WriteProtected_SentinelError(t *testing.T) {
	// Une écriture sur disque ouvert en lecture seule doit renvoyer la sentinelle
	// media.ErrWriteProtected (que le cœur traduit en erreur 72).
	p := makeFD(t, "ro.fd", sizeSingle)
	d, _ := impl.OpenDisk(p, true) // readOnly
	defer d.Close()
	err := d.WriteSector(0, 0, 1, [256]byte{})
	if !errors.Is(err, media.ErrWriteProtected) {
		t.Errorf("écriture sur disque protégé : err=%v, want media.ErrWriteProtected", err)
	}
}

func TestDisk_GeometryNoAlias(t *testing.T) {
	// Vérifie le stride : 16 secteurs/piste et 1280 secteurs/unité. Trois positions
	// distinctes doivent stocker des données distinctes (pas d'aliasing d'offset).
	d, _ := impl.OpenDisk(makeFD(t, "geo.fd", sizeDouble), false)
	defer d.Close()
	positions := []struct {
		u, p, s int
		val     byte
	}{
		{0, 0, 1, 0x11}, // tout début
		{0, 0, 2, 0x22}, // secteur suivant (+256)
		{0, 1, 1, 0x33}, // piste suivante (+16 secteurs)
		{1, 0, 1, 0x44}, // unité suivante (+1280 secteurs) — au-delà de la capacité 327680 → doit échouer
	}
	for _, pos := range positions {
		var data [256]byte
		for i := range data {
			data[i] = pos.val
		}
		err := d.WriteSector(pos.u, pos.p, pos.s, data)
		if pos.u == 1 {
			// unité 1 commence à 1280*256 = 327680 = taille du fichier → hors capacité.
			if err == nil {
				t.Errorf("unité 1 sur disque 1-unité (327680) : écriture devrait échouer (hors capacité)")
			}
			continue
		}
		if err != nil {
			t.Fatalf("écriture (u%d p%d s%d) : %v", pos.u, pos.p, pos.s, err)
		}
	}
	// Relecture : les 3 positions valides conservent des valeurs distinctes.
	for _, pos := range positions[:3] {
		got, err := d.ReadSector(pos.u, pos.p, pos.s)
		if err != nil {
			t.Fatalf("relecture (u%d p%d s%d) : %v", pos.u, pos.p, pos.s, err)
		}
		if got[0] != pos.val {
			t.Errorf("(u%d p%d s%d) = 0x%02X, want 0x%02X (aliasing d'offset ?)", pos.u, pos.p, pos.s, got[0], pos.val)
		}
	}
}
