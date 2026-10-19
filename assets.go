// Package main provides embedded assets for the Skynet GUI.
package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed images/banner.png
var bannerImage []byte

//go:embed images/icon.png
var iconImage []byte

// icon is the embedded icon as a Fyne resource.
var icon = fyne.NewStaticResource("icon.png", iconImage)
