// @ts-check

import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import classic from 'ember-classic-decorator';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';

/**
 * @typedef KeyValue
 * @type {object}
 * @property {string} key
 * @property {string} value
 */

/**
 * @typedef SecureVariable
 * @type {object}
 */

/**
 * A Secure Variable has a path, namespace, and an array of key-value pairs within the client.
 * On the server, these key-value pairs are serialized into object structure.
 * @class
 * @extends Model
 */
@classic
export default class VariableModel extends Model {
  /**
   * Can be any arbitrary string, but behaves best when used as a slash-delimited file path.
   *
   * @type {string}
   */
  @attr('string') path;

  /**
   * @type {string}
   */
  @attr('string') namespace;

  /**
   * @type {MutableArray<KeyValue>}
   */
  @attr({
    defaultValue() {
      return [{ key: '', value: '' }];
    },
  })
  keyValues;
}
