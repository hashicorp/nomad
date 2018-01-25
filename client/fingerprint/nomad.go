package fingerprint

import (
	"log"

	cstructs "github.com/hashicorp/nomad/client/structs"
)

// NomadFingerprint is used to fingerprint the Nomad version
type NomadFingerprint struct {
	StaticFingerprinter
	logger *log.Logger
}

// NewNomadFingerprint is used to create a Nomad fingerprint
func NewNomadFingerprint(logger *log.Logger) Fingerprint {
	f := &NomadFingerprint{logger: logger}
	return f
}

func (f *NomadFingerprint) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	resp.Attributes["nomad.version"] = req.Config.Version.VersionNumber()
	resp.Attributes["nomad.revision"] = req.Config.Version.Revision
	return nil
}
