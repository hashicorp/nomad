import classic from 'ember-classic-decorator';
import ApplicationSerializer from './application';

@classic
export default class VariableSerializer extends ApplicationSerializer {
  primaryKey = 'Path';
  separateNanos = ['CreateTime', 'ModifyTime'];

  normalize(typeHash, hash) {
    // hash.NamespaceID = hash.Namespace;

    // ID is a composite of both the job ID and the namespace the job is in
    hash.PlainId = hash.ID;
    hash.ID = JSON.stringify([hash.Path, hash.Namespace || 'default']);
    return super.normalize(typeHash, hash);
  }

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
    delete json.ModifyTime;
    delete json.CreateTime;
    console.log('serializing', json);
    return json;
  }
}
