import { copy } from '@ember/object/internals';
import { get } from '@ember/object';
import { makeArray } from '@ember/array';
import JSONSerializer from 'ember-data/serializers/json';
import removeRecord from '../utils/remove-record';

export default JSONSerializer.extend({
  primaryKey: 'ID',

  keyForAttribute(attr) {
    return attr.camelize().capitalize();
  },

  keyForRelationship(attr, relationshipType) {
    const key = `${attr
      .singularize()
      .camelize()
      .capitalize()}ID`;
    return relationshipType === 'hasMany' ? key.pluralize() : key;
  },

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
  },

  normalizeFindAllResponse(store, modelClass) {
    const result = this._super(...arguments);
    this.cullStore(store, modelClass.modelName, result.data);
    return result;
  },

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
  },

  modelNameFromPayloadKey(key) {
    return key.dasherize().singularize();
  },
});
