/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Model from '@ember-data/model';
import { computed } from '@ember/object';
import classic from 'ember-classic-decorator';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';
import { trimPath } from '../helpers/trim-path';
import { attr } from '@ember-data/model';

/**
 * @typedef KeyValue
 * @type {object}
 * @property {string} key
 * @property {string} value
 */

/**
 * @typedef Variable
 * @type {object}
 */

/**
 * A Variable has a path, namespace, and an array of key-value pairs within the client.
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
  @attr('string', { defaultValue: '' }) path;

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
  /** @type {Date} */
  @attr('date') createTime;
  /** @type {Date} */
  @attr('date') modifyTime;
  /** @type {string} */
  @attr('string', { defaultValue: 'default' }) namespace;

  @computed('path')
  get parentFolderPath() {
    const split = this.path.split('/');
    const [, ...folderPath] = [split.pop(), ...split];
    return folderPath.join('/');
  }

  /**
   * Removes starting and trailing slashes, pathLinkedEntitiesand sets the ID property
   */
  setAndTrimPath() {
    this.set('path', trimPath([this.path]));
    if (!this.get('id')) {
      this.set('id', `${this.get('path')}@${this.get('namespace')}`);
    }
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

  // Gets the path of the variable, and if it starts with jobs/, delimits on / and returns each part separately in an array

  /**
   * @typedef pathLinkedEntities
   * @type {Object}
   * @property {string} job
   * @property {string} [group]
   * @property {string} [task]
   */

  /**
   * @type {pathLinkedEntities}
   */
  get pathLinkedEntities() {
    const entityTypes = ['job', 'group', 'task'];
    const emptyEntities = { job: '', group: '', task: '' };
    if (
      this.path?.startsWith('nomad/jobs/') &&
      this.path?.split('/').length <= 5
    ) {
      return this.path
        .split('/')
        .slice(2, 5)
        .reduce((acc, pathPart, index) => {
          acc[entityTypes[index]] = pathPart;
          return acc;
        }, emptyEntities);
    } else {
      return emptyEntities;
    }
  }
}
