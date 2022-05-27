// @ts-check

import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import classic from 'ember-classic-decorator';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';

/**
 * @typedef SecureVariable
 * @type {object}
 * @property {string} key
 * @property {string} value
 */

@classic
export default class VariableModel extends Model {
  @attr('string') path;
  @attr('string') namespace;

  /**
   * @type {MutableArray<SecureVariable>}
   */
  @attr({
    defaultValue() {
      return [{ key: '', value: '' }];
    },
  })
  keyValues;
}
