package machine

// IRQSource identifie une source d'interruption d'une machine. Le MO5 n'utilise que
// IRQFrame ; la famille TO ajoute le timer 6846 et le clavier.
type IRQSource uint

const (
	IRQFrame    IRQSource = iota // fin de trame 50 Hz (toutes machines)
	IRQTimer                     // timer 6846 (famille TO)
	IRQKeyboard                  // clavier (famille TO)
)

// IRQLines modélise des lignes d'interruption NIVEAU-déclenchées (revue Codex,
// bloquant) : une source reste assertée jusqu'à son acquittement explicite, de sorte
// qu'une IRQ n'est PAS perdue si le drapeau I du CPU est masqué au moment de
// l'assertion (contrairement à un cpu.IRQ() ponctuel sur front).
//
// IRQLines est destiné à être détenu par le moteur (internal/engine) et échantillonné
// en frontière d'instruction : tant qu'au moins une source est assertée et que I est
// démasqué, le CPU prend l'interruption. Les machines (Device) assertent/clearent
// leurs sources depuis leur timing par-instruction.
type IRQLines struct {
	asserted uint // masque de bits indexé par IRQSource
}

// Assert positionne (maintient) la ligne d'IRQ de la source src.
func (l *IRQLines) Assert(src IRQSource) { l.asserted |= 1 << uint(src) }

// Clear relâche la ligne d'IRQ de la source src (acquittement).
func (l *IRQLines) Clear(src IRQSource) { l.asserted &^= 1 << uint(src) }

// IsAsserted indique si la source src est actuellement assertée.
func (l *IRQLines) IsAsserted(src IRQSource) bool { return l.asserted&(1<<uint(src)) != 0 }

// Pending indique si au moins une source d'IRQ est assertée.
func (l *IRQLines) Pending() bool { return l.asserted != 0 }

// Reset relâche toutes les lignes (reset matériel).
func (l *IRQLines) Reset() { l.asserted = 0 }
