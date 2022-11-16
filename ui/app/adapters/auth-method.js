// @ts-check
import { default as ApplicationAdapter, namespace } from './application';
import { dasherize } from '@ember/string';
import classic from 'ember-classic-decorator';

@classic
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
   * @property {string} AuthMethod
   * @property {string} RedirectUri
   * @property {string} ClientNonce
   * @property {Object[]} Meta // NOTE: unsure if array of objects or kv pairs
   */

  /**
   * @param {ACLOIDCAuthURLParams} params
   * @returns
   */
  getAuthURL({ AuthMethod, RedirectUri, ClientNonce, Meta }) {
    const url = `/${this.namespace}/oidc/auth-url`;
    return this.ajax(url, 'POST', {
      data: {
        AuthMethod,
        RedirectUri,
        ClientNonce,
        Meta,
      },
    });
  }
}
