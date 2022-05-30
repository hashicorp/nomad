import classic from 'ember-classic-decorator';
import ApplicationSerializer from './application';

@classic
export default class VariableSerializer extends ApplicationSerializer {
  primaryKey = 'Path';

  // Transform API's Items object into an array of a KeyValue objects
  normalizeFindRecordResponse(store, typeClass, hash, id, ...args) {
    // TODO: prevent items-less saving at API layer
    if (!hash.Items) {
      hash.Items = { '': '' };
    }
    hash.KeyValues = Object.entries(hash.Items).map(([key, value]) => {
      return {
        key,
        value,
      };
    });
    delete hash.Items;
    return super.normalizeFindRecordResponse(
      store,
      typeClass,
      hash,
      id,
      ...args
    );
  }

  // Transform our KeyValues array into an Items object
  serialize(snapshot, options) {
    const json = super.serialize(snapshot, options);
    json.Items = json.KeyValues.reduce((acc, { key, value }) => {
      acc[key] = value;
      return acc;
    }, {});
    delete json.KeyValues;
    return json;
  }
}
