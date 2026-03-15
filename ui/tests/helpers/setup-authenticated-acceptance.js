/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function setupAuthenticatedAcceptance(
  hooks,
  { withAgent = true } = {},
) {
  hooks.beforeEach(function () {
    if (withAgent && server.db.agents.length === 0) {
      server.create('agent');
    }

    const token = server.create('token');
    window.localStorage.nomadTokenSecret = token.secretId;
  });
}
