package config

type BuiltinController struct {
	Enabled bool
}

type ExternalController struct {
	Name     string
	Endpoint string
	NodePool string
}

type AdmissionControllers struct {
	// In-tree controllers

	// Out-of-tree controllers
	External []ExternalController
}
