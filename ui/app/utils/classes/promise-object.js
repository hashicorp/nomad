/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ObjectProxy from '@ember/object/proxy';
import PromiseProxyMixin from '@ember/object/promise-proxy-mixin';
import classic from 'ember-classic-decorator';

@classic
export default class PromiseObject extends ObjectProxy.extend(
  PromiseProxyMixin
) {}
