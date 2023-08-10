// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mitchellh/pointerstructure"

	"github.com/hashicorp/nomad/nomad/structs"
)

// SelectorData returns the data for go-bexpr for selector evaluation.
func SelectorData(
	am *structs.ACLAuthMethod, idClaims, userClaims map[string]interface{}) (*structs.ACLAuthClaims, error) {

	// Ensure the issuer and subscriber data does not get overwritten.
	if len(userClaims) > 0 {

		iss, issOk := idClaims["iss"]
		sub, subOk := idClaims["sub"]

		for k, v := range userClaims {
			idClaims[k] = v
		}

		if issOk {
			idClaims["iss"] = iss
		}
		if subOk {
			idClaims["sub"] = sub
		}
	}

	return extractClaims(am, idClaims)
}

// extractClaims takes the claim mapping configuration of the OIDC auth method,
// extracts the claims, and returns a map of data that can be used with
// go-bexpr.
func extractClaims(
	am *structs.ACLAuthMethod, all map[string]interface{}) (*structs.ACLAuthClaims, error) {

	values, err := extractMappings(all, am.Config.ClaimMappings)
	if err != nil {
		return nil, err
	}

	list, err := extractListMappings(all, am.Config.ListClaimMappings)
	if err != nil {
		return nil, err
	}

	return &structs.ACLAuthClaims{
		Value: values,
		List:  list,
	}, nil
}

// extractMappings extracts the string value mappings.
func extractMappings(
	all map[string]interface{}, mapping map[string]string) (map[string]string, error) {

	result := make(map[string]string)
	for source, target := range mapping {
		rawValue := getClaim(all, source)
		if rawValue == nil {
			continue
		}

		strValue, ok := stringifyClaimValue(rawValue)
		if !ok {
			return nil, fmt.Errorf("error converting claim '%s' to string from unknown type %T",
				source, rawValue)
		}

		result[target] = strValue
	}

	return result, nil
}

// extractListMappings builds a metadata map of string list values from a set
// of claims and claims mappings.  The referenced claims must be strings and
// the claims mappings must be of the structure:
//
//	{
//	    "/some/claim/pointer": "metadata_key1",
//	    "another_claim": "metadata_key2",
//	     ...
//	}
func extractListMappings(
	all map[string]interface{}, mappings map[string]string) (map[string][]string, error) {

	result := make(map[string][]string)
	for source, target := range mappings {
		rawValue := getClaim(all, source)
		if rawValue == nil {
			continue
		}

		rawList, ok := normalizeList(rawValue)
		if !ok {
			return nil, fmt.Errorf("%q list claim could not be converted to string list", source)
		}

		list := make([]string, 0, len(rawList))
		for _, raw := range rawList {
			value, ok := stringifyClaimValue(raw)
			if !ok {
				return nil, fmt.Errorf("value %v in %q list claim could not be parsed as string",
					raw, source)
			}

			if value == "" {
				continue
			}
			list = append(list, value)
		}

		result[target] = list
	}

	return result, nil
}

// getClaim returns a claim value from allClaims given a provided claim string.
// If this string is a valid JSONPointer, it will be interpreted as such to
// locate the claim. Otherwise, the claim string will be used directly.
//
// There is no fixup done to the returned data type here. That happens a layer
// up in the caller.
func getClaim(all map[string]interface{}, claim string) interface{} {
	if !strings.HasPrefix(claim, "/") {
		return all[claim]
	}

	val, err := pointerstructure.Get(all, claim)
	if err != nil {
		// We silently drop the error since keys that are invalid
		// just have no values.
		return nil
	}

	return val
}

// stringifyClaimValue will try to convert the provided raw value into a
// faithful string representation of that value per these rules:
//
// - strings      => unchanged
// - bool         => "true" / "false"
// - json.Number  => String()
// - float32/64   => truncated to int64 and then formatted as an ascii string
// - intXX/uintXX => casted to int64 and then formatted as an ascii string
//
// If successful the string value and true are returned. otherwise an empty
// string and false are returned.
func stringifyClaimValue(rawValue interface{}) (string, bool) {
	switch v := rawValue.(type) {
	case string:
		return v, true
	case bool:
		return strconv.FormatBool(v), true
	case json.Number:
		return v.String(), true
	case float64:
		// The claims unmarshalled by go-oidc don't use UseNumber, so
		// they'll come in as float64 instead of an integer or json.Number.
		return strconv.FormatInt(int64(v), 10), true

		// The numerical type cases following here are only here for the sake
		// of numerical type completion. Everything is truncated to an integer
		// before being stringified.
	case float32:
		return strconv.FormatInt(int64(v), 10), true
	case int8:
		return strconv.FormatInt(int64(v), 10), true
	case int16:
		return strconv.FormatInt(int64(v), 10), true
	case int32:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case int:
		return strconv.FormatInt(int64(v), 10), true
	case uint8:
		return strconv.FormatInt(int64(v), 10), true
	case uint16:
		return strconv.FormatInt(int64(v), 10), true
	case uint32:
		return strconv.FormatInt(int64(v), 10), true
	case uint64:
		return strconv.FormatInt(int64(v), 10), true
	case uint:
		return strconv.FormatInt(int64(v), 10), true
	default:
		return "", false
	}
}

// normalizeList takes an item or a slice and returns a slice. This is useful
// when providers are expected to return a list (typically of strings) but
// reduce it to a non-slice type when the list count is 1.
//
// There is no fixup done to elements of the returned slice here. That happens
// a layer up in the caller.
func normalizeList(raw interface{}) ([]interface{}, bool) {
	switch v := raw.(type) {
	case []interface{}:
		return v, true
	case string, // note: this list should be the same as stringifyClaimValue
		bool,
		json.Number,
		float64,
		float32,
		int8,
		int16,
		int32,
		int64,
		int,
		uint8,
		uint16,
		uint32,
		uint64,
		uint:
		return []interface{}{v}, true
	default:
		return nil, false
	}
}
