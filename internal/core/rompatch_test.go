package core

// Tests white-box du patch ROM système (rompatch.go). Assertions observables :
// on vérifie les octets effectivement présents dans m.rom[] après application.

import "testing"

// romWithOriginals fabrique une ROM 16 Ko contenant les octets d'ORIGINE attendus
// aux 5 points de patch (le reste à zéro). Simule la vraie ROM non patchée.
func romWithOriginals() []byte {
	rom := make([]byte, 0x4000)
	for _, p := range dcmo5SystemRomPatches {
		i := int(p.addr) - romBase
		rom[i] = p.original[0]
		rom[i+1] = p.original[1]
	}
	return rom
}

func romByte(m *Machine, addr uint16) byte { return m.rom[int(addr)-romBase] }

func TestRomPatch_AppliesAllPoints(t *testing.T) {
	m, err := NewMachine(Options{ROMSys: romWithOriginals(), PatchSystemROM: true})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	for _, p := range dcmo5SystemRomPatches {
		if got0, got1 := romByte(m, p.addr), romByte(m, p.addr+1); got0 != p.patched[0] || got1 != p.patched[1] {
			t.Errorf("%s @%04X : got %02X %02X, want %02X %02X (%s)",
				"patch", p.addr, got0, got1, p.patched[0], p.patched[1], p.desc)
		}
		// Cohérence du bus : Read8 doit refléter le patch.
		if v := m.Read8(p.addr); v != p.patched[0] {
			t.Errorf("Read8(%04X) = %02X, want %02X", p.addr, v, p.patched[0])
		}
	}
}

func TestRomPatch_ReportAndIdempotence(t *testing.T) {
	// ROM non patchée au départ (PatchSystemROM=false → pas de patch à l'init).
	m, _ := NewMachine(Options{ROMSys: romWithOriginals(), PatchSystemROM: false})

	rep := m.applySystemRomPatches()
	if !rep.OK || rep.Applied != len(dcmo5SystemRomPatches) || rep.Already != 0 {
		t.Fatalf("1er patch : %+v, want OK=true Applied=%d Already=0", rep, len(dcmo5SystemRomPatches))
	}
	// Réappliquer ne doit RIEN changer (idempotence).
	rep2 := m.applySystemRomPatches()
	if !rep2.OK || rep2.Applied != 0 || rep2.Already != len(dcmo5SystemRomPatches) {
		t.Fatalf("2e patch : %+v, want OK=true Applied=0 Already=%d", rep2, len(dcmo5SystemRomPatches))
	}
}

func TestRomPatch_UnknownROM_NoChange(t *testing.T) {
	// ROM dont UN point ne correspond ni à l'origine ni au patch → non reconnue.
	rom := romWithOriginals()
	bad := dcmo5SystemRomPatches[1].addr // 2e point
	rom[int(bad)-romBase] = 0x00         // octet inattendu
	rom[int(bad)-romBase+1] = 0x00

	m, _ := NewMachine(Options{ROMSys: rom, PatchSystemROM: true})

	// Tout-ou-rien : AUCUN point ne doit avoir été modifié, y compris les valides.
	for _, p := range dcmo5SystemRomPatches {
		if p.addr == bad {
			if got := romByte(m, p.addr); got != 0x00 {
				t.Errorf("point inconnu @%04X modifié : %02X (devait rester intact)", p.addr, got)
			}
			continue
		}
		if got := romByte(m, p.addr); got != p.original[0] {
			t.Errorf("ROM non reconnue : point @%04X patché à tort (%02X), tout-ou-rien violé", p.addr, got)
		}
	}
}

func TestRomPatch_OptOutLeavesRomIntact(t *testing.T) {
	m, _ := NewMachine(Options{ROMSys: romWithOriginals(), PatchSystemROM: false})
	for _, p := range dcmo5SystemRomPatches {
		if got := romByte(m, p.addr); got != p.original[0] {
			t.Errorf("PatchSystemROM=false : @%04X modifié (%02X), ROM devait rester intacte", p.addr, got)
		}
	}
}
