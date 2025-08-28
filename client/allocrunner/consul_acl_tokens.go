// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

/*
func decodeACLToken(b64ACLToken string, token *consulapi.ACLToken) error {
	decodedBytes, err := base64.StdEncoding.DecodeString(b64ACLToken)
	if err != nil {
		return fmt.Errorf("unable to process ACLToken: %w", err)
	}

	if len(decodedBytes) != 0 {
		if err := json.Unmarshal(decodedBytes, token); err != nil {
			return fmt.Errorf("unable to unmarshal ACLToken: %w", err)
		}
	}

	return nil
}

func encodeACLToken(token *consulapi.ACLToken) (string, error) {
	jsonBytes, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("unable to marshal ACL token: %w", err)
	}

	return base64.StdEncoding.EncodeToString(jsonBytes), nil
}

func (m *WIDMgr) Set(swi *structs.SignedWorkloadIdentity) error {
	storedIdentities, err := m.db.GetAllocIdentities(m.allocID)
	if err != nil {
		return err
	}

	index := slices.IndexFunc(storedIdentities, func(i *structs.SignedWorkloadIdentity) bool {
		return i.IdentityName == swi.IdentityName
	})

	if index == -1 {
		return errors.New("workload identity not found")
	}

	storedIdentities[index] = swi

	return m.db.PutAllocIdentities(m.allocID, storedIdentities)
}
*/
