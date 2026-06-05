# Fidelity Suite — DCMO5 Moderne

> Date : 2026-06-05. Périmètre : non-régression déterministe du portage.

## Principe

L'invariant fondamental de la fidelity suite :

**Même ROM + même nombre de cycles → même état machine (RAM, framebuffer).**

Aucune dépendance sur le temps réel ni sur l'OS. Les tests passent identiquement
sur macOS et Linux.

## ROM de test

**Toutes les ROM utilisées dans les tests sont générées par code** — aucune
ROM Thomson MO5 copyright n'est utilisée ni embarquée.

| ROM | Contenu | Utilisation |
|---|---|---|
| `makeNOPROM()` | NOP infini (0x12), vecteurs → 0xC000 | Déterminisme RAM, framebuffer |
| `makeCounterROM()` | LDA/INCA/STA/BRA loop, compteur à 0x4000 | Cycle-count, comptage itérations |

## Scénarios couverts (`internal/core/fidelity_test.go`)

### Déterminisme RAM

| Test | Invariant |
|---|---|
| `RAMChecksum_Deterministic` | 2 runs identiques → même FNV-32 de RAM |
| `RAMChecksum_DifferentCycles` | 15 vs 30 cycles → checksums différents (non trivial) |
| `CounterROM_Increment` | 15 cycles → RAM[0x4000]=1 ; 30 → RAM[0x4000]=2 |

### Déterminisme framebuffer

| Test | Invariant |
|---|---|
| `FramebufferChecksum_Deterministic` | Même RAM vidéo → même checksum pixels |
| `FramebufferChecksum_RAMChange` | Modifier RAM vidéo → checksum change |

### CPU golden (avancés)

| Test | Invariant |
|---|---|
| `NOPBurst_CycleCount` | 100 cycles demandés → 100–102 consommés (NOP=2) |
| `CounterROM_20Iterations` | 300 cycles → compteur=20 (15 cycles/iter) |
| `Reset_ClearsState` | Step + Reset → RAM revenue au pattern init |
| `FrameWidth_Pixels` | Framebuffer = FrameWidth × FrameHeight pixels |

## Définition régression bloquante

Une **régression bloquante** est tout changement qui :

1. **Modifie un checksum RAM ou framebuffer** pour une séquence ROM+cycles documentée.
2. **Casse un invariant de cycle-count** (ex: NOP ne consomme plus 2 cycles).
3. **Brise le déterminisme** (deux runs identiques donnent des résultats différents).
4. **Panique ou segfault** sur tout opcode valide 6809.

## Écarts acceptés (documentés)

| Écart | Justification |
|---|---|
| Opcodes illégaux 6809 → NOP implicite | Comportement non spécifié ; même stratégie que la ref C |
| Cycles en excès de Step() ≤ durée max instruction (17 cycles PSHS) | Instruction non interruptible |
| Timing IRQ non encore couplé à videolinecycle | Implémenté en P7+ |

## Couverture future (non couverte en P7)

- Tests avec ROM réelle Thomson (nécessite validation droits — voir `DESIGN/LICENSING.md`).
- Comparaison cycle-à-cycle avec DCMO5 v11 (nécessite harness d'extraction).
- Timing vidéo IRQ/VBL.
- Audio (P7 scope optionnel).
