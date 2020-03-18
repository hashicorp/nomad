import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    hash.NamespaceID = hash.Namespace;

    hash.PlainID = hash.ID;

    // TODO This shouldn't hardcode `csi/` as part of the ID,
    // but it is necessary to make the correct find request and the
    // payload does not contain the required information to derive
    // this identifier.
    hash.ID = JSON.stringify([`csi/${hash.ID}`, hash.NamespaceID || 'default']);

    return this._super(typeHash, hash);
  },
});
