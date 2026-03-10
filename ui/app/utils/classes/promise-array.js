/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ArrayProxy from '@ember/array/proxy';
import PromiseProxyMixin from '@ember/object/promise-proxy-mixin';
import classic from 'ember-classic-decorator';

@classic
export default class PromiseArray extends ArrayProxy.extend(
  PromiseProxyMixin
) {}
