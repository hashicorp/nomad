package fingerprint

func initPlatformFingerprints(fps map[string]Factory) {
	fps["cgroup"] = NewCGroupFingerprint
	fps["bridge"] = NewBridgeFingerprint
}
