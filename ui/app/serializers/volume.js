import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    hash.NamespaceID = hash.Namespace;

    hash.PlainId = hash.ID;

    // TODO These shouldn't hardcode `csi/` as part of the IDs,
    // but it is necessary to make the correct find requests and the
    // payload does not contain the required information to derive
    // this identifier.
    hash.ID = JSON.stringify([`csi/${hash.ID}`, hash.NamespaceID || 'default']);
    hash.PluginID = `csi/${hash.PluginID}`;

    return this._super(typeHash, hash);
  },
});
