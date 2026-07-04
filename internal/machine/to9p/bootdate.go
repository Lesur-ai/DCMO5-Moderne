package to9p

import (
	"bytes"
	"time"
)

const (
	bootDateStrOff  = 0xEB90 // chaîne « jj-mm-aa » (addr BASIC $2B90)
	bootDateTermOff = 0xEB98 // octet terminal (0x1F) suivant la chaîne
	bootDateInitOff = 0xE4E2 // routine de reset à patcher (addr BASIC $24E2)
)

var (
	bootDatePlaceholder = []byte("jj-mm-aa")
	bootDateInitOrig    = []byte{0xb7, 0xe7, 0xfe, 0xb7, 0xe7, 0xfa}
	bootDateInitPatch   = []byte{0x8e, 0x2b, 0x90, 0xbd, 0x29, 0xc8}
)

const bootDateLayout = "02-01-06"

func injectBootDate(romBasic []byte, now time.Time) bool {
	if bootDateInitOff+len(bootDateInitPatch) > len(romBasic) || bootDateTermOff >= len(romBasic) {
		return false
	}
	if !bytes.Equal(romBasic[bootDateStrOff:bootDateStrOff+len(bootDatePlaceholder)], bootDatePlaceholder) ||
		romBasic[bootDateTermOff] != 0x1f ||
		!bytes.Equal(romBasic[bootDateInitOff:bootDateInitOff+len(bootDateInitOrig)], bootDateInitOrig) {
		return false
	}
	copy(romBasic[bootDateStrOff:], []byte(now.Format(bootDateLayout)))
	romBasic[bootDateTermOff] = 0x1f
	copy(romBasic[bootDateInitOff:], bootDateInitPatch)
	return true
}
