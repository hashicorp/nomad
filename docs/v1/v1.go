// Package v1 nomad.
//
// Documentation of the Nomad v1 API.
//  TermsOfService: https://www.hashicorp.com/terms-of-service
//	Schemes: http, https
//  BasePath: /v1/
//  Version: 1.0.0
//  Host: localhost:4646
//  Consumes:
//  - application/json
//  Produces:
//  - application/json
//  Contact: support@hashicorp.com
//  License: MIT 2 https://github.com/hashicorp/nomad/blob/main/LICENSE
//  Security:
//  - apiKey
//  SecurityDefinitions:
//  apiKey:
//   type: apiKey
//   description: ACL tokens to operate. The token are used to authenticate the request and determine if the request is allowed based on the associated authorizations. Tokens are specified per-request by using the `X-Nomad-Token` request header set to the `SecretID` of an ACL Token.
//   name: X-Nomad-Token
//   in: header
//
// swagger:meta
package v1