package consensus

import (
	"fmt"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/crypto"
)

// ValidatorSetFromGenesis maps genesis initialValidators to a ValidatorSet.
func ValidatorSetFromGenesis(g *config.Genesis) (*ValidatorSet, error) {
	if g == nil {
		return nil, fmt.Errorf("consensus: nil genesis")
	}
	if len(g.InitialValidators) == 0 {
		return nil, fmt.Errorf("consensus: genesis has no initial validators")
	}
	vals := make([]Validator, 0, len(g.InitialValidators))
	for _, iv := range g.InitialValidators {
		addr, err := crypto.HexToAddress(iv.Address)
		if err != nil {
			return nil, fmt.Errorf("consensus: validator address %q: %w", iv.Address, err)
		}
		power := iv.VotingPower
		if power == 0 {
			power = 1
		}
		vals = append(vals, Validator{Address: addr, Power: power})
	}
	return NewValidatorSet(vals)
}