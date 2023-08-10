/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function setPolicy(policy) {
  const { id: policyId } = server.create('policy', policy);
  const clientToken = server.create('token', { type: 'client' });
  clientToken.policyIds = [policyId];
  clientToken.save();

  window.localStorage.clear();
  window.localStorage.nomadTokenSecret = clientToken.secretId;
}
