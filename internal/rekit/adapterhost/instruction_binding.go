package adapterhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/productioninstruction"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimeinstruction"
)

func validateAdapterInstructionBinding(caseRoot, pack string, identity *instructionpacket.Identity) error {
	pack = strings.TrimSpace(pack)
	_, production := productioninstruction.ContractFor(pack)
	if !production {
		if identity != nil {
			return fmt.Errorf("non-production adapter cannot claim a production instruction identity")
		}
		return nil
	}
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve adapter instruction state root: %w", err)
	}
	if identity == nil {
		if root.Legacy {
			return nil
		}
		return fmt.Errorf("production adapter omitted its durable instruction identity")
	}
	if err := productioninstruction.ValidateIdentity(pack, *identity); err != nil {
		return fmt.Errorf("adapter instruction identity is invalid: %w", err)
	}
	if _, err := runtimeinstruction.Reload(caseRoot, pack, *identity); err != nil {
		return fmt.Errorf("adapter instruction identity is not current: %w", err)
	}
	return nil
}

func cloneAdapterInstructionIdentity(identity *instructionpacket.Identity) *instructionpacket.Identity {
	if identity == nil {
		return nil
	}
	clone := instructionpacket.CloneIdentity(*identity)
	return &clone
}

func equalAdapterInstructionIdentity(left, right *instructionpacket.Identity) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return instructionpacket.EqualIdentity(*left, *right)
}

func decodeAdapterInstructionIdentityJSON(value string) (*instructionpacket.Identity, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(value)))
	decoder.DisallowUnknownFields()
	var identity instructionpacket.Identity
	if err := decoder.Decode(&identity); err != nil {
		return nil, fmt.Errorf("decode instruction identity: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("instruction identity must contain exactly one JSON object")
	}
	if err := instructionpacket.ValidateIdentity(identity); err != nil {
		return nil, fmt.Errorf("instruction identity is invalid: %w", err)
	}
	return cloneAdapterInstructionIdentity(&identity), nil
}

func marshalAdapterInstructionIdentityJSON(identity *instructionpacket.Identity) (string, error) {
	if identity == nil {
		return "", nil
	}
	data, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func appendAdapterInstructionIdentityArg(args []string, identity *instructionpacket.Identity) ([]string, error) {
	value, err := marshalAdapterInstructionIdentityJSON(identity)
	if err != nil {
		return nil, err
	}
	if value == "" {
		return args, nil
	}
	return append(args, "-instruction-identity-json", value), nil
}

func validateAdapterChildInstructionIdentity(expected, actual *instructionpacket.Identity) error {
	if !equalAdapterInstructionIdentity(expected, actual) {
		return fmt.Errorf("adapter child instruction identity does not match the parent binding")
	}
	if actual != nil {
		if err := instructionpacket.ValidateIdentity(*actual); err != nil {
			return fmt.Errorf("adapter child instruction identity is invalid: %w", err)
		}
	}
	return nil
}
