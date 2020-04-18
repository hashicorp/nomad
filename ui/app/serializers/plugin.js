import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    hash.PlainID = hash.ID;

    // TODO This shouldn't hardcode `csi/` as part of the ID,
    // but it is necessary to make the correct find request and the
    // payload does not contain the required information to derive
    // this identifier.
    hash.ID = `csi/${hash.ID}`;

    hash.Nodes = hash.Nodes || [];
    hash.Controllers = hash.Controllers || [];

    return this._super(typeHash, hash);
  },
});
