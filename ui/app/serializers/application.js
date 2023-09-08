/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { copy } from 'ember-copy';
import { get } from '@ember/object';
import { makeArray } from '@ember/array';
import JSONSerializer from '@ember-data/serializer/json';
import { pluralize, singularize } from 'ember-inflector';
import removeRecord from '../utils/remove-record';
import { assign } from '@ember/polyfills';
import classic from 'ember-classic-decorator';
import { camelize, capitalize, dasherize } from '@ember/string';
@classic
export default class Application extends JSONSerializer {
  primaryKey = 'ID';

  /**
    A list of keys that are converted to empty arrays if their value is null.

    arrayNullOverrides = ['Array'];
    { Array: null } => { Array: [] }

    @property arrayNullOverrides
    @type String[]
   */
  arrayNullOverrides = null;

  /**
    A list of keys that are converted to empty objects if their value is null.

    objectNullOverrides = ['Object'];
    { Object: null } => { Object: {} }

    @property objectNullOverrides
    @type String[]
   */
  objectNullOverrides = null;

  /**
    A list of keys or objects to convert a map into an array of maps with the original map keys as Name properties.

    mapToArray = ['Map'];
    { Map: { a: { x: 1 } } } => { Map: [ { Name: 'a', x: 1 }] }

    mapToArray = [{ beforeName: 'M', afterName: 'Map' }];
    { M: { a: { x: 1 } } } => { Map: [ { Name: 'a', x: 1 }] }

    @property mapToArray
    @type (String|Object)[]
   */
  mapToArray = null;

  /**
    A list of keys for nanosecond timestamps that will be split into two properties: `separateNanos = ['Time']` will
    produce a `Time` property with a millisecond timestamp and `TimeNanos` with the nanoseconds alone.

    separateNanos = ['Time'];
    { Time: 1607839992000100000 } => { Time: 1607839992000, TimeNanos: 100096 }

    @property separateNanos
    @type String[]
   */
  separateNanos = null;

  keyForAttribute(attr) {
    return capitalize(camelize(attr));
  }

  keyForRelationship(attr, relationshipType) {
    const key = `${capitalize(camelize(singularize(attr)))}ID`;
    return relationshipType === 'hasMany' ? pluralize(key) : key;
  }

  // Modeled after the pushPayload for ember-data/serializers/rest
  pushPayload(store, payload) {
    const documentHash = {
      data: [],
      included: [],
    };

    Object.keys(payload).forEach((key) => {
      const modelName = this.modelNameFromPayloadKey(key);
      const serializer = store.serializerFor(modelName);
      const type = store.modelFor(modelName);

      makeArray(payload[key]).forEach((hash) => {
        const { data, included } = serializer.normalize(type, hash, key);
        documentHash.data.push(data);
        if (included) {
          documentHash.included.push(...included);
        }
      });
    });

    store.push(documentHash);
  }

  normalize(modelClass, hash) {
    if (hash) {
      if (this.arrayNullOverrides) {
        this.arrayNullOverrides.forEach((key) => {
          if (!hash[key]) {
            hash[key] = [];
          }
        });
      }
      if (this.objectNullOverrides) {
        this.objectNullOverrides.forEach((key) => {
          if (!hash[key]) {
            hash[key] = {};
          }
        });
      }

      if (this.mapToArray) {
        this.mapToArray.forEach((conversion) => {
          let apiKey, uiKey;

          if (conversion.beforeName) {
            apiKey = conversion.beforeName;
            uiKey = conversion.afterName;
          } else {
            apiKey = conversion;
            uiKey = conversion;
          }

          const map = hash[apiKey] || {};

          hash[uiKey] = Object.keys(map)
            .sort()
            .map((mapKey) => {
              const propertiesForKey = map[mapKey] || {};
              const convertedMap = { Name: mapKey };

              assign(convertedMap, propertiesForKey);

              return convertedMap;
            });
        });
      }

      if (this.separateNanos) {
        this.separateNanos.forEach((key) => {
          const timeWithNanos = hash[key];
          hash[`${key}Nanos`] = timeWithNanos % 1000000;
          hash[key] = Math.floor(timeWithNanos / 1000000);
        });
      }
    }

    return super.normalize(modelClass, hash);
  }

  normalizeFindAllResponse(store, modelClass) {
    const result = super.normalizeFindAllResponse(...arguments);
    this.cullStore(store, modelClass.modelName, result.data);
    return result;
  }

  // When records are removed server-side, and therefore don't show up in requests,
  // the local copies of those records need to be unloaded from the store.
  cullStore(store, type, records, storeFilter = () => true) {
    const newRecords = copy(records).filter((record) => get(record, 'id'));
    const oldRecords = store.peekAll(type);
    oldRecords
      .filter((record) => get(record, 'id'))
      .filter(storeFilter)
      .forEach((old) => {
        const newRecord = newRecords.find(
          (record) => get(record, 'id') === get(old, 'id')
        );
        if (!newRecord) {
          removeRecord(store, old);
        } else {
          newRecords.removeObject(newRecord);
        }
      });
  }

  modelNameFromPayloadKey(key) {
    return singularize(dasherize(key));
  }
}
