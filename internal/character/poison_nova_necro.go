package character

import (
	"log/slog"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
)

const (
	// Positioning constants
	PoisonNovaMinDistance = 4
	PoisonNovaMaxDistance = 12
)

type PoisonNovaNecro struct {
	BaseCharacter
}

// CheckKeyBindings ensures that all the required skills for the build are bound to a key.
func (p PoisonNovaNecro) CheckKeyBindings() []skill.ID {
	requiredKeybindings := []skill.ID{
		skill.PoisonNova,
		skill.LowerResist,
		skill.AmplifyDamage,
		skill.CorpseExplosion,
		skill.BoneArmor,
		skill.Revive,
		skill.TomeOfTownPortal,
	}

	// Teleport is optional, but recommended
	if p.CharacterCfg.Character.UseTeleport {
		requiredKeybindings = append(requiredKeybindings, skill.Teleport)
	}

	missingKeybindings := []skill.ID{}

	for _, s := range requiredKeybindings {
		if _, found := p.Data.KeyBindings.KeyBindingForSkill(s); !found {
			missingKeybindings = append(missingKeybindings, s)
		}
	}

	if len(missingKeybindings) > 0 {
		p.Logger.Debug("There are missing required key bindings.", slog.Any("Bindings", missingKeybindings))
	}

	return missingKeybindings
}

// BuffSkills defines the skills that should be cast as buffs.
func (p PoisonNovaNecro) BuffSkills() []skill.ID {
	// Bone Armor is the primary buff for this build.
	return []skill.ID{skill.BoneArmor}
}

// PreCTABuffSkills defines skills to be cast before switching to CTA.
func (p PoisonNovaNecro) PreCTABuffSkills() []skill.ID {
	return []skill.ID{}
}

// KillMonsterSequence defines the main attack rotation for the Poison Nova Necromancer.
func (p PoisonNovaNecro) KillMonsterSequence(
	monsterSelector func(d game.Data) (data.UnitID, bool),
	skipOnImmunities []stat.Resist,
) error {
	ctx := context.Get()

	id, found := monsterSelector(*p.Data)
	if !found {
		return nil // No monsters found
	}

	for {
		ctx.PauseIfNotPriority()

		if !p.preBattleChecks(id, skipOnImmunities) {
			return nil
		}

		monster, found := p.Data.Monsters.FindByID(id)
		if !found || monster.Stats[stat.Life] <= 0 {
			return nil // Monster is already dead or gone
		}

		// 1. Cast Lower Resist
		if err := step.SecondaryAttack(skill.LowerResist, monster.UnitID, 1, step.Distance(5, 10)); err != nil {
			p.Logger.Warn("Failed to cast Lower Resist", slog.String("error", err.Error()))
		}

		// 2. Cast Poison Nova
		if err := step.SecondaryAttack(skill.PoisonNova, monster.UnitID, 2, step.Distance(PoisonNovaMinDistance, PoisonNovaMaxDistance)); err != nil {
			p.Logger.Warn("Failed to cast Poison Nova", slog.String("error", err.Error()))
		}

		// 3. Actively wait for a corpse to appear
		var corpse data.Monster
		foundCorpse := false
		for i := 0; i < 16; i++ { // Wait for up to 4 seconds (16 * 250ms)
			corpse, foundCorpse = p.findCorpse()
			if foundCorpse {
				break
			}
			time.Sleep(time.Millisecond * 250)
		}

		// 4. If a corpse was found, start the chain reaction
		if foundCorpse {
			// 5. Cast Amplify Damage near the corpse to affect nearby enemies
			step.SecondaryAttack(skill.AmplifyDamage, monster.UnitID, 1, step.Distance(5, 10))

			// 6. Spam Corpse Explosion
			for i := 0; i < 10; i++ {
				step.SecondaryAttack(skill.CorpseExplosion, corpse.UnitID, 1)
				time.Sleep(time.Millisecond * 250)

				// Check if there are any enemies left nearby, if not, we can stop exploding
				enemiesNearby := false
				for _, m := range p.Data.Monsters.Enemies() {
					if m.Stats[stat.Life] > 0 {
						enemiesNearby = true
						break
					}
				}
				if !enemiesNearby {
					break
				}
			}
		}

		// 7. Revive minions after the fight
		p.reviveMinions()

		// Check if there are still monsters to kill for the next loop iteration
		id, found = monsterSelector(*p.Data)
		if !found {
			return nil
		}
	}
}

// findCorpse looks for the first available corpse on the screen.
func (p PoisonNovaNecro) findCorpse() (data.Monster, bool) {
	for _, monster := range p.Data.Monsters.Enemies() {
		if monster.Stats[stat.Life] <= 0 {
			return monster, true
		}
	}

	return data.Monster{}, false
}

// reviveMinions casts Revive on available corpses.
func (p PoisonNovaNecro) reviveMinions() {
	petCount := 0
	for _, m := range p.Data.Monsters {
		if m.IsPet() {
			petCount++
		}
	}

	// We want to have 10 revives.
	if petCount < 10 {
		for _, monster := range p.Data.Monsters.Enemies() {
			if monster.Stats[stat.Life] <= 0 { // Is a corpse
				step.SecondaryAttack(skill.Revive, monster.UnitID, 1)
				time.Sleep(time.Millisecond * 300) // Pause to allow revive animation
			}
		}
	}
}

// killMonsterByName is a generic function to kill a specific monster.
func (p PoisonNovaNecro) killMonsterByName(id npc.ID, monsterType data.MonsterType, skipOnImmunities []stat.Resist) error {
	return p.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		if m, found := d.Monsters.FindOne(id, monsterType); found {
			return m.UnitID, true
		}
		return 0, false
	}, skipOnImmunities)
}

// KillAndariel implements the logic to kill Andariel.
func (p PoisonNovaNecro) KillAndariel() error {
	return p.killMonsterByName(npc.Andariel, data.MonsterTypeUnique, nil)
}

// KillDuriel implements the logic to kill Duriel.
func (p PoisonNovaNecro) KillDuriel() error {
	return p.killMonsterByName(npc.Duriel, data.MonsterTypeUnique, nil)
}

// KillMephisto implements the logic to kill Mephisto.
func (p PoisonNovaNecro) KillMephisto() error {
	return p.killMonsterByName(npc.Mephisto, data.MonsterTypeUnique, nil)
}

// KillDiablo implements the logic to kill Diablo.
func (p PoisonNovaNecro) KillDiablo() error {
	return p.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		diablo, found := d.Monsters.FindOne(npc.Diablo, data.MonsterTypeUnique)
		if !found {
			return 0, false
		}
		return diablo.UnitID, true
	}, nil)
}

// KillBaal implements the logic to kill Baal.
func (p PoisonNovaNecro) KillBaal() error {
	return p.killMonsterByName(npc.BaalCrab, data.MonsterTypeUnique, nil)
}

// KillCountess implements the logic to kill the Countess.
func (p PoisonNovaNecro) KillCountess() error {
	return p.killMonsterByName(npc.DarkStalker, data.MonsterTypeSuperUnique, nil)
}

// KillSummoner implements the logic to kill the Summoner.
func (p PoisonNovaNecro) KillSummoner() error {
	return p.killMonsterByName(npc.Summoner, data.MonsterTypeUnique, nil)
}

// KillIzual implements the logic to kill Izual.
func (p PoisonNovaNecro) KillIzual() error {
	return p.killMonsterByName(npc.Izual, data.MonsterTypeUnique, nil)
}

// KillCouncil implements the logic to kill the High Council.
func (p PoisonNovaNecro) KillCouncil() error {
	return p.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		for _, m := range d.Monsters.Enemies() {
			if m.Name == npc.CouncilMember || m.Name == npc.CouncilMember2 || m.Name == npc.CouncilMember3 {
				return m.UnitID, true
			}
		}
		return 0, false
	}, nil)
}

// KillPindle implements the logic to kill Pindleskin.
func (p PoisonNovaNecro) KillPindle() error {
	return p.killMonsterByName(npc.DefiledWarrior, data.MonsterTypeSuperUnique, p.CharacterCfg.Game.Pindleskin.SkipOnImmunities)
}

// KillNihlathak implements the logic to kill Nihlathak.
func (p PoisonNovaNecro) KillNihlathak() error {
	return p.killMonsterByName(npc.Nihlathak, data.MonsterTypeSuperUnique, nil)
}
