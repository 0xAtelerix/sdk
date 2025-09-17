package gosdk

type ValidatorSet struct {
	Set map[ValidatorID]Stake `cbor:"1,keyasint"`
}

func NewValidatorSet(ms ...map[ValidatorID]Stake) *ValidatorSet {
	var m map[ValidatorID]Stake

	if len(ms) > 0 {
		m = ms[0]
	} else {
		m = make(map[ValidatorID]Stake)
	}

	return &ValidatorSet{
		Set: m,
	}
}

func (vs *ValidatorSet) GetStake(id ValidatorID) Stake {
	return vs.Set[id]
}

type ValidatorID uint32

type Stake uint64
