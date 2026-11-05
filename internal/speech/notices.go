package speech

import (
	_ "embed"
	"fmt"
	"strings"
)

// Licensing for what the app redistributes. Each embedded file is the exact
// upstream text; they travel with the source so the packaged app and its
// attribution screen never drift from what is actually shipped.

//go:embed notices/sherpa-onnx-Apache-2.0.txt
var sherpaLicense string

//go:embed notices/onnxruntime-MIT.txt
var onnxRuntimeLicense string

//go:embed notices/moonshine-MIT.txt
var moonshineLicense string

// Component is one redistributed piece and its license.
type Component struct {
	// Name is the component's project name.
	Name string
	// License is the SPDX-ish short name shown in a list.
	License string
	// Homepage is where the component lives.
	Homepage string
	// Text is the full license text.
	Text string
}

// Components returns every third-party piece the app ships, with its license
// text. The engine binary (sherpa-onnx), its inference runtime (onnxruntime)
// and the model (Moonshine) are three separate licenses, so each is listed on
// its own.
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

// Notices renders every component's license into one THIRD-PARTY-NOTICES text,
// the file the installer drops beside the app so the terms travel with the
// binaries even when the app is never opened.
func Notices() string {
	var b strings.Builder
	b.WriteString("Zowe Worship — third-party notices\n")
	b.WriteString("This build redistributes the speech engine below. Their licenses follow.\n")

	for _, c := range Components() {
		fmt.Fprintf(&b, "\n\n%s\n%s\n%s — %s\n%s\n\n", strings.Repeat("=", 78), c.Name, c.License, c.Homepage, strings.Repeat("=", 78))
		b.WriteString(strings.TrimRight(c.Text, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}
