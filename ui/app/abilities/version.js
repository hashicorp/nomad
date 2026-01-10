/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

export default class Version extends AbstractAbility {
  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'specificNamespaceSupportsTagging'
  )
  canTag;

  @computed('rulesForNamespace.@each.capabilities')
  get specificNamespaceSupportsTagging() {
    return (
      this.namespaceIncludesCapability('submit-job') ||
      this.namespaceIncludesCapability('tag-job-version')
    );
  }
}