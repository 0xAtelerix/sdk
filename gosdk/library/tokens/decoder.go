package tokens

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

// DecodeEventInto decodes a log that matches `eventName` in `a` into a user struct T.
// T must be a struct (or pointer to struct when passed to mapper) whose fields are tagged with `abi:"<name>"`
// (or the field name is used if the tag is missing).
// It returns (zero T, matched=false) if the signature doesn't match.
func DecodeEventInto[T any](
	a abi.ABI,
	eventName string,
	lg *gethtypes.Log,
) (out T, matched bool, err error) {
	ev, ok := a.Events[eventName]
	if !ok {
		return out, false, fmt.Errorf("%w: %s", ErrABIUnknownEvent, eventName)
	}

	if len(lg.Topics) == 0 || lg.Topics[0] != ev.ID {
		// signature doesn't match this event
		return out, false, nil
	}

	// Validate indexed topics count FIRST (important for overloaded events)
	idxArgs := indexed(ev.Inputs)
	if len(lg.Topics)-1 != len(idxArgs) {
		// treat as matched signature, but wrong variant
		// return out, true, fmt.Errorf("%w: parse topics for %s: want %d indexed topics, got %d",
		// 	ErrABIIncorrectNumberOfTopics, eventName, len(idxArgs), len(lg.Topics)-1)
		// Better: signal "not matched" so the registry can try next candidate:
		return out, false, nil
	}

	// Unpack non-indexed data only after we know the candidate fits
	// TODO: memoization!
	nonIdx := ev.Inputs.NonIndexed()

	// TODO: memoization!
	nonVals, err := nonIdx.Unpack(lg.Data)
	if err != nil {
		return out, true, fmt.Errorf("unpack data for %s: %w", eventName, err)
	}

	// Decode indexed values
	// TODO: memoization!
	idxVals := make([]any, len(idxArgs))
	for i, arg := range idxArgs {
		topic := lg.Topics[i+1]

		switch arg.Type.T {
		case abi.AddressTy:
			// address is right-aligned in 32 bytes
			idxVals[i] = common.BytesToAddress(topic.Bytes()[12:])

		case abi.UintTy, abi.IntTy:
			// topics are 32-byte big-endian words
			idxVals[i] = new(big.Int).SetBytes(topic.Bytes())

		case abi.BoolTy:
			idxVals[i] = new(big.Int).SetBytes(topic.Bytes()).Cmp(big.NewInt(0)) != 0

		case abi.HashTy, abi.FixedBytesTy:
			// 32-byte fixed types can be returned as the hash itself
			idxVals[i] = topic

		default:
			// dynamic indexed types (string/bytes/arrays) are hashed in topics;
			// you cannot recover the original value. Return the hash.
			idxVals[i] = topic
		}
	}

	// Merge and assign
	// Build name -> value map (lowercased)
	// TODO: memoization!
	values := make(map[string]any, len(ev.Inputs))
	for i, arg := range nonIdx {
		values[strings.ToLower(arg.Name)] = nonVals[i]
	}

	// TODO: memoization!
	for i, arg := range idxArgs {
		values[strings.ToLower(arg.Name)] = idxVals[i]
	}

	// Assign into T by ABI names / struct tags (reflection only here; decoding used ABI)
	var dst T

	// TODO: memoization!
	if err := assignByABI(&dst, ev.Inputs, values); err != nil {
		return out, true, err
	}

	return dst, true, nil
}

// DecodeAndMap decodes into T then maps (T, Meta) -> R with user-provided mapper.
// If the log doesn't match the event, matched=false and zero R is returned.
func DecodeAndMap[T any, R any](
	a abi.ABI,
	eventName string,
	lg *gethtypes.Log,
	txHash common.Hash,
	mapper func(T, Meta) (R, error),
) (R, bool, error) {
	var zeroR R

	ev, matched, err := DecodeEventInto[T](a, eventName, lg)
	if !matched || err != nil {
		return zeroR, matched, err
	}

	meta := Meta{
		Contract: lg.Address,
		TxHash:   txHash,
		LogIndex: lg.Index,
	}
	r, mapErr := mapper(ev, meta)

	return r, true, mapErr
}
