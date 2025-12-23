// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import (
	"testing"

	"github.com/a-h/templ"
)

func TestMsgKeyAsComponent(t *testing.T) {
	var _ templ.Component = MsgKey("foo")
}
