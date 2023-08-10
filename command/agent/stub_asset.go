// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ui
// +build !ui

package agent

import (
	assetfs "github.com/elazarl/go-bindata-assetfs"
)

func init() {
	uiEnabled = false
	stubHTML = `<!DOCTYPE html>
<html>
<p>Nomad UI is not available in this binary. To get Nomad UI do one of the following:</p>
<ul>
<li><a href="https://www.nomadproject.io/downloads.html">Download an official release</a></li>
<li>Run <pre>make release</pre> to create your own release binaries.
<li>Run <pre>make dev-ui</pre> to create a development binary with the UI.
</ul>
</html>
`
}

// assetFS is a stub for building Nomad without a UI.
func assetFS() *assetfs.AssetFS {
	return nil
}
