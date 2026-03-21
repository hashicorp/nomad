/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';

export default class Deployment extends AbstractAbility {
  get canFail() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.specificNamespaceSupportsFailing
    );
  }

  get canPromote() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.specificNamespaceSupportsPromoting
    );
  }

  get specificNamespaceSupportsFailing() {
    return (
      this.namespaceIncludesCapability('submit-job') ||
      this.namespaceIncludesCapability('fail-deployment')
    );
  }

  get specificNamespaceSupportsPromoting() {
    return (
      this.namespaceIncludesCapability('submit-job') ||
      this.namespaceIncludesCapability('promote-deployment')
    );
  }
}
