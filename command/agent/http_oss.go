// +build !pro,!ent

package agent

// registerEnterpriseHandlers is a no-op for the oss release
func (s *HTTPServer) registerEnterpriseHandlers() {}
