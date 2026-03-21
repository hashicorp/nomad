/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';

export default class Version extends AbstractAbility {
  get canTag() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.specificNamespaceSupportsTagging
    );
  }

  get specificNamespaceSupportsTagging() {
    return (
      this.namespaceIncludesCapability('submit-job') ||
      this.namespaceIncludesCapability('tag-job-version')
    );
  }
}
