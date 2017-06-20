import JSONSerializer from 'ember-data/serializers/json';

export default JSONSerializer.extend({
  primaryKey: 'ID',

  keyForAttribute(attr) {
    return attr.camelize().capitalize();
  },
});
