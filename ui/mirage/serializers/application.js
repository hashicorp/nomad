import { RestSerializer } from 'ember-cli-mirage';

const keyCase = str =>
  str === 'id'
    ? 'ID'
    : str
        .camelize()
        .capitalize()
        .replace(/Id/g, 'ID');

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

  keyForCollection: keyCase,
  keyForAttribute: keyCase,
  keyForRelationship: keyCase,
  keyForRelationshipIds: keyCase,
  keyForEmbeddedRelationship: keyCase,
});
