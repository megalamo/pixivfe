// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import (
	"context"
	"io"
)

// Translatable is a value that can translate itself using a context.
// Types such as [MsgKey] implement Translatable.
type Translatable interface {
	Tr(ctx context.Context) string
}

// MsgKey is a source message id (msgid) string.
//
// Construct with MsgKey("Are you sure you want to quit?") and call Tr(ctx) to resolve
// using the current locale in ctx.
//
// MsgKey should be the original English UI text, not an invented key.
type MsgKey string

// Tr translates this msgid within the current locale chain.
// It is equivalent to calling [Tr] with the same msgid.
// The ctx may be nil, in which case the base locale is used.
// Setup must be called successfully before using this.
func (s MsgKey) Tr(ctx context.Context) string {
	return Tr(ctx, string(s))
}

func (s MsgKey) Render(ctx context.Context, w io.Writer) error {
	_, err := io.WriteString(w, s.Tr(ctx))

	return err
}
