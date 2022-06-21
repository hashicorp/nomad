// @ts-check
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import classic from 'ember-classic-decorator';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';
import { trimPath } from '../helpers/trim-path';

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
   * @type {MutableArray<KeyValue>}
   */
  @attr({
    defaultValue() {
      return [{ key: '', value: '' }];
    },
  })
  keyValues;

  /** @type {number} */
  @attr('number') createIndex;
  /** @type {number} */
  @attr('number') modifyIndex;
  /** @type {string} */
  @attr('string') createTime;
  /** @type {string} */
  @attr('string') modifyTime;
  /** @type {string} */
  @attr('string') namespace;

  @computed('path')
  get parentFolderPath() {
    const split = this.path.split('/');
    const [, ...folderPath] = [split.pop(), ...split];
    return folderPath.join('/');
  }

  /**
   * Removes starting and trailing slashes, and sets the ID property
   */
  setAndTrimPath() {
    this.set('path', trimPath([this.path]));
    this.set('id', this.get('path'));
  }

  /**
   * Translates the key-value pairs into an object structure.
   */
  @computed('keyValues')
  get items() {
    return this.keyValues.reduce((acc, { key, value }) => {
      acc[key] = value;
      return acc;
    }, {});
  }
}
