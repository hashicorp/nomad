import Ember from 'ember';
import JSONSerializer from 'ember-data/serializers/json';

const { makeArray } = Ember;

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

      store.push(documentHash);
    });
  },

  modelNameFromPayloadKey(key) {
    return key.dasherize().singularize();
  },
});
