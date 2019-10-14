package main

/*
#cgo LDFLAGS: -s -w
#include "vips.h"
*/
import "C"

import (
	"fmt"
	"regexp"
)

type urlOptions map[string][]string

type imageType int

const (
	imageTypeUnknown = imageType(C.UNKNOWN)
	imageTypeJPEG    = imageType(C.JPEG)
	imageTypePNG     = imageType(C.PNG)
	imageTypeWEBP    = imageType(C.WEBP)
	imageTypeGIF     = imageType(C.GIF)
	imageTypeICO     = imageType(C.ICO)
	imageTypeSVG     = imageType(C.SVG)
)

type processingHeaders struct {
	Accept        string
	Width         string
	ViewportWidth string
	DPR           string
}

var imageTypes = map[string]imageType{
	"jpeg": imageTypeJPEG,
	"jpg":  imageTypeJPEG,
	"png":  imageTypePNG,
	"webp": imageTypeWEBP,
	"gif":  imageTypeGIF,
	"ico":  imageTypeICO,
	"svg":  imageTypeSVG,
}

type gravityType int

const (
	gravityCenter gravityType = iota
	gravityNorth
	gravityEast
	gravitySouth
	gravityWest
	gravityNorthWest
	gravityNorthEast
	gravitySouthWest
	gravitySouthEast
	gravitySmart
	gravityFocusPoint
)

var gravityTypes = map[string]gravityType{
	"ce":   gravityCenter,
	"no":   gravityNorth,
	"ea":   gravityEast,
	"so":   gravitySouth,
	"we":   gravityWest,
	"nowe": gravityNorthWest,
	"noea": gravityNorthEast,
	"sowe": gravitySouthWest,
	"soea": gravitySouthEast,
	"sm":   gravitySmart,
	"fp":   gravityFocusPoint,
}

type gravityOptions struct {
	Type gravityType
	X, Y float64
}

type resizeType int

const (
	resizeFit resizeType = iota
	resizeFill
	resizeCrop
)

var resizeTypes = map[string]resizeType{
	"fit":  resizeFit,
	"fill": resizeFill,
	"crop": resizeCrop,
}

type rgbColor struct{ R, G, B uint8 }

var hexColorRegex = regexp.MustCompile("^([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$")

const (
	hexColorLongFormat  = "%02x%02x%02x"
	hexColorShortFormat = "%1x%1x%1x"
)

type watermarkOptions struct {
	Enabled   bool
	Opacity   float64
	Replicate bool
	Gravity   gravityType
	OffsetX   int
	OffsetY   int
	Scale     float64
}

type processingOptions struct {
	Resize     resizeType
	Width      int
	Height     int
	Dpr        float64
	Gravity    gravityOptions
	Enlarge    bool
	Expand     bool
	Format     imageType
	Quality    int
	Flatten    bool
	Background rgbColor
	Blur       float32
	Sharpen    float32

	CacheBuster string

	Watermark watermarkOptions

	UsedPresets []string
}

func (it imageType) String() string {
	for k, v := range imageTypes {
		if v == it {
			return k
		}
	}
	return ""
}

func (gt gravityType) String() string {
	for k, v := range gravityTypes {
		if v == gt {
			return k
		}
	}
	return ""
}

func (rt resizeType) String() string {
	for k, v := range resizeTypes {
		if v == rt {
			return k
		}
	}
	return ""
}

func colorFromHex(hexcolor string) (rgbColor, error) {
	c := rgbColor{}

	if !hexColorRegex.MatchString(hexcolor) {
		return c, fmt.Errorf("Invalid hex color: %s", hexcolor)
	}

	if len(hexcolor) == 3 {
		fmt.Sscanf(hexcolor, hexColorShortFormat, &c.R, &c.G, &c.B)
		c.R *= 17
		c.G *= 17
		c.B *= 17
	} else {
		fmt.Sscanf(hexcolor, hexColorLongFormat, &c.R, &c.G, &c.B)
	}

	return c, nil
}
