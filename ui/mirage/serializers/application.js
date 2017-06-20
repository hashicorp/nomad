import { RestSerializer } from 'ember-cli-mirage';

export default RestSerializer.extend({
  serialize() {
    const json = RestSerializer.prototype.serialize.apply(this, arguments);
    const keys = Object.keys(json);
    if (keys.length === 1) {
      return json[keys[0]];
    } else {
      return json;
    }
  },

  keyForAttribute(attr) {
    if (attr === 'id') {
      return 'ID';
    }
    return attr.camelize().capitalize();
  },
});
