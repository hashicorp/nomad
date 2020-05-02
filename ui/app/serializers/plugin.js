import ApplicationSerializer from './application';

// Convert a map[string]interface{} into an array of objects
// where the key becomes a property at propKey.
// This is destructive. The original object is mutated to avoid
// excessive copies of the originals which would otherwise just
// be garbage collected.
const unmap = (hash, propKey) =>
  Object.keys(hash).map(key => {
    const record = hash[key];
    record[propKey] = key;
    return record;
  });

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    hash.PlainID = hash.ID;

    // TODO This shouldn't hardcode `csi/` as part of the ID,
    // but it is necessary to make the correct find request and the
    // payload does not contain the required information to derive
    // this identifier.
    hash.ID = `csi/${hash.ID}`;

    const nodes = hash.Nodes || {};
    const controllers = hash.Controllers || {};

    hash.Nodes = unmap(nodes, 'NodeID');
    hash.Controllers = unmap(controllers, 'NodeID');

    return this._super(typeHash, hash);
  },
});
