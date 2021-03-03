package rpcmock

import (
	"context"
	"crypto/rand"

	"golang.org/x/xerrors"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/internal/crypto"
)

// InvokeRaw implements tg.Invoker.
func (i *Mock) InvokeRaw(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
	h := i.Handler()

	id, err := crypto.RandInt64(rand.Reader)
	if err != nil {
		return xerrors.Errorf("generate id: %w", err)
	}

	body, err := h(id, input)
	if err != nil {
		return xerrors.Errorf("mock invoke: %w", err)
	}

	buf := new(bin.Buffer)
	if err := body.Encode(buf); err != nil {
		return xerrors.Errorf("encode: %w", err)
	}
	if err := output.Decode(buf); err != nil {
		return xerrors.Errorf("decode: %w", err)
	}
	return nil
}
