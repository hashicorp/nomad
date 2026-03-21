/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { default as ApplicationAdapter, namespace } from './application';
import { dasherize } from '@ember/string';

export default class AuthMethodAdapter extends ApplicationAdapter {
  namespace = `${namespace}/acl`;

  /**
   * @param {string} modelName
   * @returns {string}
   */
  urlForFindAll(modelName) {
    return dasherize(this.buildURL(modelName));
  }

  /**
   * @typedef {Object} ACLOIDCAuthURLParams
   * @property {string} AuthMethodName
   * @property {string} RedirectUri
   * @property {string} ClientNonce
   * @property {Object[]} Meta // NOTE: unsure if array of objects or kv pairs
   */

  /**
   * @param {ACLOIDCAuthURLParams} params
   * @returns
   */
  getAuthURL({ AuthMethodName, RedirectUri, ClientNonce, Meta }) {
    const url = `/${this.namespace}/oidc/auth-url`;
    return this.ajax(url, 'POST', {
      data: {
        AuthMethodName,
        RedirectUri,
        ClientNonce,
        Meta,
      },
    });
  }
}
