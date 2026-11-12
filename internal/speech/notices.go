package speech

import (
	_ "embed"
	"fmt"
	"strings"
)

// Embedded license files are the exact upstream text, so the packaged app and
// its attribution screen never drift from what is actually shipped.

//go:embed notices/sherpa-onnx-Apache-2.0.txt
var sherpaLicense string

//go:embed notices/onnxruntime-MIT.txt
var onnxRuntimeLicense string

//go:embed notices/moonshine-MIT.txt
var moonshineLicense string

type Component struct {
	Name string
	// License is the SPDX short name shown in a list.
	License  string
	Homepage string
	Text     string
}

// The engine, its inference runtime and the model are three separate licenses,
// so each is listed on its own.
func Components() []Component {
	return []Component{
		{
			Name:     "sherpa-onnx",
			License:  "Apache-2.0",
			Homepage: "https://github.com/k2-fsa/sherpa-onnx",
			Text:     sherpaLicense,
		},
		{
			Name:     "ONNX Runtime",
			License:  "MIT",
			Homepage: "https://github.com/microsoft/onnxruntime",
			Text:     onnxRuntimeLicense,
		},
		{
			Name:     "Moonshine (models)",
			License:  "MIT",
			Homepage: "https://github.com/moonshine-ai/moonshine",
			Text:     moonshineLicense,
		},
	}
}

// Notices renders the THIRD-PARTY-NOTICES text the installer drops beside the
// app, so the terms travel with the binaries even when it is never opened.
func Notices() string {
	var b strings.Builder
	b.WriteString("Zopresent — third-party notices\n")
	b.WriteString("This build redistributes the speech engine below. Their licenses follow.\n")

	for _, c := range Components() {
		fmt.Fprintf(&b, "\n\n%s\n%s\n%s — %s\n%s\n\n", strings.Repeat("=", 78), c.Name, c.License, c.Homepage, strings.Repeat("=", 78))
		b.WriteString(strings.TrimRight(c.Text, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}
