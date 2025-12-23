// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
Package core makes requests to Pixiv APIs and parses information into structured data.

You may use this package independently as follows:

	package main

	import (
		"fmt"
		"net/http"
		"time"

		"codeberg.org/pixivfe/pixivfe/v3/core"
		"codeberg.org/pixivfe/pixivfe/v3/core/tokenmanager"
	)

	func main() {
		tokenmanager.DefaultTokenManager = tokenmanager.NewTokenManager(
			[]string{"YOUR_TOKEN_HERE"},
			5,
			1000*time.Millisecond,
			32000*time.Millisecond,
			"round-robin",
		)
		fake_request, err := http.NewRequest("GET", "/", nil)
		if err != nil {
			panic(err)
		}
		data, err := core.GetNovelPageData(fake_request, "24253567")
		if err != nil {
			panic(err)
		}
		fmt.Println(data)
	}

This package's API is ever changing, so please pin a specific version of this package if you want to use it in your program.
*/
package core
