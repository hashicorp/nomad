import { get, computed } from '@ember/object';
import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import flat from 'npm:flat';

const { unflatten } = flat;

export default Fragment.extend({
  attributes: attr(),

  attributesStructured: computed('attributes', function() {
    const original = this.get('attributes');

    if (!original) {
      return;
    }

    // `unflatten` doesn't sort keys before unflattening, so manual preprocessing is necessary.
    const attrs = Object.keys(original)
      .sort()
      .reduce((obj, key) => {
        obj[key] = original[key];
        return obj;
      }, {});
    return unflatten(attrs, { overwrite: true });
  }),

  unknownProperty(key) {
    // Returns the exact value in index 0 and the subtree in index 1
    //
    // ex: nodeAttrs.get('driver.docker')
    // [ "1", { version: "17.05.0-ce", volumes: { enabled: "1" } } ]
    return get(this.get('attributesStructured'), key);
  },
});
