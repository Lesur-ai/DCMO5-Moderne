package gatearray

// joystick.go — Inc J1a du support joystick TO8D. État des deux manettes,
// publié par la couche hôte (App→Host) et lu par le CPU 6809 émulé via les
// registres mémoire 0xe7cc (direction) et 0xe7cd (action), avec mux contrôlé
// par les bits 2 des ports e7ce/e7cf (cf. readIO).
//
// Convention bits LOGIQUE INVERSÉE, identique MO5 (ref C dcmo5emulation.c
// Joysemul cases 0-9 byte-identique à dcto8demulation.c) :
//
//   Position : bit 0 = J1 nord (0=appuyé)     bit 4 = J2 nord
//              bit 1 = J1 sud                 bit 5 = J2 sud
//              bit 2 = J1 ouest               bit 6 = J2 ouest
//              bit 3 = J1 est                 bit 7 = J2 est
//   Action   : bit 6 = J1 bouton fire (0=appuyé)
//              bit 7 = J2 bouton fire
//              bits 0..5 = inutilisés (toujours 1 au repos)
//
// Repos = (0xFF, 0xC0). Au boot (hardReset), ces valeurs sont restaurées :
// taper Reset DOIT remettre le joystick neutre, même si l'hôte n'a pas
// republié d'état. Cf. machine.NeutralJoystick (Inc J0).

// SetJoystick publie l'état idempotent des deux manettes. Appelée à chaque
// tick depuis Host.tick (Inc J2a) via l'adapter TO8D. La valeur écrasée :
// aucune accumulation, aucun front détecté (le code utilisateur lit le port
// directement via 0xe7cc/0xe7cd). Sûre depuis n'importe quelle goroutine
// SI Host.tick reste mono-thread (invariant existant).
func (g *GateArray) SetJoystick(position, action uint8) {
	g.joysPosition = position
	g.joysAction = action
}
