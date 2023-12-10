/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default class OTTExchangeError extends Error {
  message = 'Failed to exchange the one-time token.';
}
