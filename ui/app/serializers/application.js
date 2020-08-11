import { copy } from 'ember-copy';
import { get } from '@ember/object';
import { makeArray } from '@ember/array';
import JSONSerializer from 'ember-data/serializers/json';
import { pluralize, singularize } from 'ember-inflector';
import removeRecord from '../utils/remove-record';
import { assign } from '@ember/polyfills';

export default class Application extends JSONSerializer {
  primaryKey = 'ID';

  keyForAttribute(attr) {
    return attr.camelize().capitalize();
  }

  keyForRelationship(attr, relationshipType) {
    const key = `${singularize(attr)
      .camelize()
      .capitalize()}ID`;
    return relationshipType === 'hasMany' ? pluralize(key) : key;
  }

  // Modeled after the pushPayload for ember-data/serializers/rest
  pushPayload(store, payload) {
    const documentHash = {
      data: [],
      included: [],
    };

    Object.keys(payload).forEach(key => {
      const modelName = this.modelNameFromPayloadKey(key);
      const serializer = store.serializerFor(modelName);
      const type = store.modelFor(modelName);

      makeArray(payload[key]).forEach(hash => {
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
    if (this.arrayNullOverrides) {
      this.arrayNullOverrides.forEach(key => {
        if (!hash[key]) {
          hash[key] = [];
        }
      });
    }

    if (this.mapToArray) {
      this.mapToArray.forEach(conversion => {
        let apiKey, uiKey;

        if (conversion.APIName) {
          apiKey = conversion.APIName;
          uiKey = conversion.UIName;
        } else if (conversion.name) {
          apiKey = conversion.name;
          uiKey = conversion.name;
        } else {
          apiKey = conversion;
          uiKey = conversion;
        }

        const map = hash[apiKey] || {};

        hash[uiKey] = Object.keys(map).map(mapKey => {
          const propertiesForKey = map[mapKey] || {};
          const convertedMap = { Name: mapKey };

          if (conversion.convertor) {
            conversion.convertor(propertiesForKey, convertedMap);
          } else {
            assign(convertedMap, propertiesForKey);
          }

          return convertedMap;
        });
      });
    }

    if (this.separateNanos) {
      this.separateNanos.forEach(key => {
        const timeWithNanos = hash[key];
        hash[`${key}Nanos`] = timeWithNanos % 1000000;
        hash[key] = Math.floor(timeWithNanos / 1000000);
      });
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
    const newRecords = copy(records).filter(record => get(record, 'id'));
    const oldRecords = store.peekAll(type);
    oldRecords
      .filter(record => get(record, 'id'))
      .filter(storeFilter)
      .forEach(old => {
        const newRecord = newRecords.find(record => get(record, 'id') === get(old, 'id'));
        if (!newRecord) {
          removeRecord(store, old);
        } else {
          newRecords.removeObject(newRecord);
        }
      });
  }

  modelNameFromPayloadKey(key) {
    return singularize(key.dasherize());
  }
}
