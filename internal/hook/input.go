package hook

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/legostin/constitution/pkg/types"
)

// ReadInput parses HookInput from the given reader (typically os.Stdin).
func ReadInput(r io.Reader) (*types.HookInput, error) {
	var input types.HookInput
	dec := json.NewDecoder(r)
	if err := dec.Decode(&input); err != nil {
		return nil, fmt.Errorf("failed to parse hook input: %w", err)
	}
	return &input, nil
}
