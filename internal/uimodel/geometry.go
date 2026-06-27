package uimodel

import "github.com/Lesur-ai/dcmo5/internal/machine"

// DisplayGeometry calcule, pour une famille de machine et la taille du framebuffer
// logique (fw,fh fixés par FrameSize()), deux repères distincts :
//
//   - la taille LOGIQUE (logW,logH) : le repère rendu par ebiten.Game.Layout, donc
//     aussi celui dans lequel Ebitengine exprime le curseur ;
//   - la taille FENÊTRE (winW,winH) : les pixels écran passés à SetWindowSize.
//
// Le but est de corriger le ratio d'aspect des familles à pixels NON carrés. Le
// gate-array (TO8/TO8D) a un framebuffer 672×216 (42 segments × 16 px en largeur,
// 8+200+8 lignes en hauteur) : ses pixels sont deux fois plus hauts que larges. On
// étire donc verticalement ×2 au niveau logique (672×432, aspect ≈ 1.555) au lieu
// de doubler les deux axes comme pour le MO5. Le MO5 reste STRICTEMENT inchangé :
// logique = framebuffer 336×216, fenêtre = ×2 = 672×432.
//
// Fonction PURE (aucune dépendance Ebitengine) : c'est le contrat testé en CI ; le
// câblage runtime (Layout/SetWindowSize) vit dans internal/app, hors CI.
func DisplayGeometry(family machine.Family, fw, fh int) (logW, logH, winW, winH int) {
	switch family {
	case machine.FamilyMO:
		// Pixels ~carrés : logique = framebuffer, fenêtre = ×2 sur les deux axes.
		return fw, fh, 2 * fw, 2 * fh
	case machine.FamilyTOGateArray:
		// Pixels deux fois plus hauts que larges : on étire ×2 en hauteur au niveau
		// logique ; la fenêtre épouse 1:1 ce repère logique (pas de sur-échelle).
		return fw, 2 * fh, fw, 2 * fh
	case machine.FamilyTO7:
		// PROVISOIRE : aucun profil TO7 n'est constructible aujourd'hui (boot CLI =
		// MO5, launcher = profils enregistrés sans TO7). On retient la géométrie
		// MO-like par défaut, à réévaluer lorsqu'un profil TO7 réel atterrira.
		return fw, fh, 2 * fw, 2 * fh
	default:
		// Valeur de Family hors énumération : erreur de programmation impossible à
		// ignorer silencieusement (cf. revue Codex Inc 3a).
		panic("uimodel.DisplayGeometry: famille de machine inconnue")
	}
}

// CursorToFramebuffer convertit un curseur exprimé dans le repère LOGIQUE (celui de
// Layout, donc des coordonnées renvoyées par ebiten.CursorPosition()) vers le repère
// FRAMEBUFFER attendu par la machine pour le crayon optique (Host.SetInput →
// SetPointer). Pour les familles à pixels carrés (MO) c'est l'identité ; pour le
// gate-array (logique étiré ×2 en hauteur), la composante Y est ramenée à l'échelle
// du framebuffer (y/2).
//
// À N'UTILISER QUE pour le crayon : un overlay/menu rendu en repère Layout doit
// faire son hit-test sur le curseur BRUT, sans cette conversion.
func CursorToFramebuffer(family machine.Family, fw, fh, x, y int) (fbX, fbY int) {
	logW, logH, _, _ := DisplayGeometry(family, fw, fh)
	return x * fw / logW, y * fh / logH
}

// EmulatorLayoutSize donne la taille LOGIQUE que ebiten.Game.Layout doit renvoyer en
// mode émulateur, selon que l'overlay (lot #117 Inc 3b) est ouvert ou non :
//
//   - overlay FERMÉ : repère d'affichage habituel de la famille (DisplayGeometry logique),
//     qu'Ebitengine met à l'échelle de la fenêtre — comportement actuel, inchangé ;
//   - overlay OUVERT : repère FENÊTRE réel (outW,outH), pour que l'UI ebitenui se
//     positionne et se dessine au pixel près. Le framebuffer gelé est alors redessiné
//     dessous en aspect-fit (cf. FramebufferAspectFit), l'overlay par-dessus.
//
// Fonction PURE (testée en CI) ; l'appel vit dans App.Layout (internal/app, hors CI).
func EmulatorLayoutSize(overlayOpen bool, family machine.Family, fw, fh, outW, outH int) (w, h int) {
	if overlayOpen {
		return outW, outH
	}
	logW, logH, _, _ := DisplayGeometry(family, fw, fh)
	return logW, logH
}

// FramebufferAspectFit calcule le rectangle de destination (x,y,w,h) où dessiner le
// framebuffer (fw×fh) CENTRÉ dans une surface outW×outH, en préservant le ratio d'aspect
// d'AFFICHAGE de la famille — celui de DisplayGeometry (logW:logH), PAS le ratio brut du
// framebuffer : le gate-array a un framebuffer 672×216 mais un aspect d'affichage 672×432.
// Le reste de la surface est du letterbox (barres) à la charge de l'appelant.
//
// Usage : overlay ouvert, Layout passe au repère fenêtre réel (cf. EmulatorLayoutSize) et
// l'émulation est gelée ; on redessine le dernier framebuffer en aspect-fit sous l'overlay.
// L'appelant applique Scale(w/fw, h/fh) puis Translate(x,y) (échelle par axe : le gate-array
// étire ainsi ×2 en hauteur, comme en plein écran).
//
// Fonction PURE (testée en CI) ; le câblage Draw vit dans internal/app (hors CI).
func FramebufferAspectFit(family machine.Family, fw, fh, outW, outH int) (x, y, w, h int) {
	logW, logH, _, _ := DisplayGeometry(family, fw, fh)
	if logW <= 0 || logH <= 0 || outW <= 0 || outH <= 0 {
		return 0, 0, 0, 0 // surface ou géométrie dégénérée : rien à dessiner
	}
	// Plus grand rectangle d'aspect logW:logH tenant dans outW×outH. Comparaison en
	// produits croisés (entiers, pas de flottant) : outW/outH ⋛ logW/logH.
	if outW*logH <= outH*logW {
		w = outW // contraint en largeur ; hauteur réduite à l'aspect
		h = outW * logH / logW
	} else {
		h = outH // contraint en hauteur ; largeur réduite à l'aspect
		w = outH * logW / logH
	}
	x = (outW - w) / 2
	y = (outH - h) / 2
	return x, y, w, h
}
